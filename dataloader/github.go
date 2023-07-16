package dataloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
	"github.com/shurcooL/githubv4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const requestReleasesPerPage = 50

var ErrRateLimit = errors.New("Reached GitHub API rate limits")

type GithubReleaseLoaderConfig struct {
	MongoStore *mongostore.Store
	Message    *mq.GithubRepoRequestMessage
}

type GithubReleaseLoader struct {
	ghClient *githubv4.Client
	store    *mongostore.Store
	repoInfo queryBriefRepo
	owner    string
	repo     string
	logger   zerolog.Logger
}

type repoInfoFromStore struct {
	ID         primitive.ObjectID `bson:"_id"`
	ExternalID string             `bson:"external_id"`
	EndCursor  *string            `bson:"end_cursor"`
}

type releaseFetcher func(ctx context.Context, afterRelease *string) ([]*mongostore.Release, string, error)

func NewGithubReleaseLoader(config GithubReleaseLoaderConfig) *GithubReleaseLoader {
	return &GithubReleaseLoader{
		ghClient: nil,
		store:    config.MongoStore,
		owner:    config.Message.Owner,
		repo:     config.Message.Repo,
		logger: log.
			With().
			Str("owner", config.Message.Owner).
			Str("repo", config.Message.Repo).
			Logger(),
	}
}

// Dispatch bound RabbitMQ job message.
func (loader *GithubReleaseLoader) Dispatch(ctx context.Context) error {
	loader.logger.Info().Msg("Message dispatch started")

	authorizedHTTPClient := oauth2.NewClient(
		context.Background(),
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_GQL_OAUTH_TOKEN")}),
	)
	loader.ghClient = githubv4.NewClient(authorizedHTTPClient)

	err := loader.ghClient.Query(ctx, &loader.repoInfo, map[string]any{
		"owner": githubv4.String(loader.owner),
		"name":  githubv4.String(loader.repo),
	})
	if err != nil {
		return err
	}

	externalID := fmt.Sprintf("github/%d", loader.repoInfo.Repository.DatabaseId)
	mongoRepoInfo, err := loader.getRepoFromStore(ctx, bson.D{{"external_id", externalID}})
	if err != nil {
		return err
	}

	defer func() {
		loader.store.
			Database(mongostore.DatabaseName).
			Collection(mongostore.SourcesCollectionName).
			UpdateOne(
				ctx,
				bson.D{{"external_id", externalID}},
				bson.D{{"$set", bson.D{{"is_fetching", false}}}},
			)
	}()

	releases, endCursor, err := loader.loadReleasesOrTags(ctx, mongoRepoInfo.EndCursor)

	if err != nil && len(releases) == 0 {
		loader.logger.Error().Err(err).Msgf("Releases loading error: %s", err)
		return err
	}

	if len(releases) == 0 {
		loader.logger.Info().Msg("New releases and tags not found, skipping db update")
		return nil
	}

	loader.logger.Info().Msgf("UpdateOne, ready to push %d new items", len(releases))

	updateResult, err := loader.store.
		Database(mongostore.DatabaseName).
		Collection(mongostore.SourcesCollectionName).
		UpdateOne(
			ctx,
			bson.D{
				{"external_id", externalID},
				{"end_cursor", mongoRepoInfo.EndCursor},
			},
			bson.D{
				{"$set", bson.D{
					{"end_cursor", endCursor},
					{"is_fetching", false},
				}},
				{"$push", bson.D{{"releases", bson.D{
					{"$each", releases},
					{"$sort", bson.D{{"published_at", 1}}},
				}}}},
			},
		)

	if updateResult.ModifiedCount == 0 {
		loader.logger.Info().Msg("Update was not commited")
	}

	return err
}

func (loader *GithubReleaseLoader) loadReleasesOrTags(ctx context.Context, afterCursor *string) ([]*mongostore.Release, *string, error) {
	loader.logger.Info().Msg("Loading releases started")

	var fetch releaseFetcher

	if loader.repoInfo.Repository.Releases.TotalCount > 0 {
		fetch = loader.loadReleases
	} else if loader.repoInfo.Repository.Refs.TotalCount > 0 {
		fetch = loader.loadTags
	} else {
		loader.logger.Info().Msg("Repository does not have any releases or tags")
		return nil, nil, nil
	}

	allReleases := []*mongostore.Release{}
	currCursor := afterCursor

	for {
		time.Sleep(1 * time.Second)

		releases, endCursor, err := fetch(ctx, currCursor)
		if err != nil {
			return allReleases, currCursor, err
		}

		if len(releases) == 0 {
			break // All releases have been fetched
		}

		for _, release := range releases {
			release.ParseTagName()

			if !release.IsPrerelease {
				allReleases = append(allReleases, release)
			}
		}

		currCursor = &endCursor
	}

	return allReleases, currCursor, nil
}

func (loader *GithubReleaseLoader) loadReleases(ctx context.Context, afterCursor *string) ([]*mongostore.Release, string, error) {
	if afterCursor != nil {
		loader.logger.Info().Msgf("Loading releases after cursor = '%s'", *afterCursor)
	} else {
		loader.logger.Info().Msgf("Loading releases from the beginning")
	}

	var githubReleasesInfo queryReleases
	err := loader.ghClient.Query(ctx, &githubReleasesInfo, map[string]any{
		"perPage":      githubv4.Int(requestReleasesPerPage),
		"afterRelease": (*githubv4.String)(afterCursor),
		"owner":        githubv4.String(loader.owner),
		"name":         githubv4.String(loader.repo),
		"order": githubv4.ReleaseOrder{
			Field:     "CREATED_AT",
			Direction: "ASC",
		},
	})
	if err != nil {
		return nil, "", err
	}

	releases := make([]*mongostore.Release, 0, len(githubReleasesInfo.Repository.Releases.Nodes))

	for _, release := range githubReleasesInfo.Repository.Releases.Nodes {
		releases = append(releases, &mongostore.Release{
			ID:          fmt.Sprint(release.ID),
			Name:        string(release.Name),
			TagName:     string(release.TagName),
			URL:         string(release.URL),
			PublishedAt: release.PublishedAt.Time,
		})
	}

	if githubReleasesInfo.RateLimit.Remaining == 0 {
		err = ErrRateLimit
	}

	return releases, string(githubReleasesInfo.Repository.Releases.PageInfo.EndCursor), err
}

func (loader *GithubReleaseLoader) loadTags(ctx context.Context, afterCursor *string) ([]*mongostore.Release, string, error) {
	if afterCursor != nil {
		loader.logger.Info().Msgf("Loading tags after cursor = '%s'", *afterCursor)
	} else {
		loader.logger.Info().Msgf("Loading tags from the beginning")
	}

	var githubTagsInfo queryTags
	err := loader.ghClient.Query(ctx, &githubTagsInfo, map[string]any{
		"perPage":      githubv4.Int(requestReleasesPerPage),
		"afterRelease": (*githubv4.String)(afterCursor),
		"owner":        githubv4.String(loader.owner),
		"name":         githubv4.String(loader.repo),
		"order": githubv4.RefOrder{
			Field:     "TAG_COMMIT_DATE",
			Direction: "ASC",
		},
	})
	if err != nil {
		return nil, "", err
	}

	tags := make([]*mongostore.Release, 0, len(githubTagsInfo.Repository.Refs.Nodes))

	for _, node := range githubTagsInfo.Repository.Refs.Nodes {
		tags = append(tags, &mongostore.Release{
			ID:          fmt.Sprint(node.ID),
			Name:        string(node.Name),
			TagName:     string(node.Name),
			URL:         string(node.Tag.CommitURL),
			PublishedAt: node.Tag.Tagger.Date.Time,
		})
	}

	if githubTagsInfo.RateLimit.Remaining == 0 {
		err = ErrRateLimit
	}

	return tags, string(githubTagsInfo.Repository.Refs.PageInfo.EndCursor), err
}

func (loader *GithubReleaseLoader) getRepoFromStore(ctx context.Context, filter bson.D) (*repoInfoFromStore, error) {
	source := new(repoInfoFromStore)
	err := loader.store.
		Database(mongostore.DatabaseName).
		Collection(mongostore.SourcesCollectionName).
		FindOne(ctx, filter, options.FindOne().SetProjection(bson.D{
			{"_id", true},
			{"external_id", true},
			{"end_cursor", true},
		})).
		Decode(source)
	if err != nil {
		return nil, err
	}

	return source, nil
}
