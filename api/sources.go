package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/labstack/echo/v4"
	"github.com/lesnoi-kot/versions-backend/common"
	"github.com/lesnoi-kot/versions-backend/mongostore"
	"github.com/lesnoi-kot/versions-backend/mq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BriefSourceDTO struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	URL         string             `bson:"url" json:"url"`
	IsFetching  bool               `bson:"is_fetching" json:"isFetching"`
}

func (api *APIService) getSources(c echo.Context) error {
	q := c.QueryParams()
	count := parseQueryParamInt(q.Get("count"), 10)
	page := parseQueryParamInt(q.Get("page"), 0)
	name := q.Get("name")

	filters := bson.D{}
	if name != "" {
		filters = append(filters, bson.E{"name", primitive.Regex{sanitizeNameFilter(name), "i"}})
	}

	sources, err := mongostore.GetDocuments(
		c.Request().Context(),
		api.Store,
		mongostore.GetDocumentsOptions[BriefSourceDTO]{
			Collection: mongostore.SourcesCollectionName,
			Filter:     filters,
			FindOptions: options.
				Find().
				SetSkip(int64(page * count)).
				SetLimit(int64(count)).
				SetMaxTime(5 * time.Second),
		},
	)
	if err != nil {
		return err
	}

	totalCount, err := api.Store.GetDocumentsCount(
		c.Request().Context(),
		mongostore.SourcesCollectionName,
		filters,
	)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{
		"totalCount": totalCount,
		"data":       sources,
	})
}

func (api *APIService) getSource(c echo.Context) error {
	sourceID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		return echo.ErrBadRequest
	}

	source, err := api.Store.GetSourceBy(
		c.Request().Context(),
		bson.D{{"_id", sourceID}},
		options.FindOne().SetProjection(bson.D{
			{"_id", true},
			{"created_at", true},
			{"updated_at", true},
			{"owner", true},
			{"name", true},
			{"description", true},
			{"url", true},
			{"is_fetching", true},
			{"releases", bson.D{
				{"$cond", bson.D{
					{"if",
						bson.D{{"$ne", bson.A{"$is_fetching", true}}},
					},
					{"then", "$releases"},
					{"else", nil},
				}},
			}},
		}),
	)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return echo.ErrNotFound
	} else if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, source)
}

func (api *APIService) addSource(c echo.Context) error {
	link := c.FormValue("link")
	owner, repo, err := common.ParseGithubRepoLink(link)
	if err != nil {
		return echo.ErrBadRequest
	}

	ctx := c.Request().Context()
	ghClient := github.NewClient(&http.Client{Timeout: 5 * time.Second})
	repoInfo, _, err := ghClient.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if _, isRateLimitError := err.(*github.RateLimitError); isRateLimitError {
			return echo.ErrServiceUnavailable
		}

		return err
	}

	externalID := fmt.Sprintf("github/%d", repoInfo.GetID())

	sess, err := api.Store.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)

	source, err := sess.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		mongoResult := api.Store.
			Database(mongostore.DatabaseName).
			Collection(mongostore.SourcesCollectionName).
			FindOneAndUpdate(
				sessCtx,
				bson.D{{"external_id", externalID}},
				bson.D{
					{"$set", bson.D{
						{"external_id", externalID},
						{"url", repoInfo.GetHTMLURL()},
						{"name", repoInfo.GetName()},
						{"owner", repoInfo.GetOwner().GetLogin()},
						{"description", repoInfo.GetDescription()},
						{"updated_at", time.Now()},
						{"is_fetching", true},
					}},
					{"$setOnInsert", bson.D{
						{"releases", []mongostore.Release{}},
						{"created_at", time.Now()},
						{"end_cursor", (*string)(nil)},
					}},
				},
				options.FindOneAndUpdate().
					SetUpsert(true).
					SetReturnDocument(options.After).
					SetProjection(bson.D{
						{"releases", false},
					}),
			)

		source := new(mongostore.Source)
		if err = mongoResult.Decode(source); err != nil {
			return nil, err
		}

		err = api.MQ.PushRepoRequest(ctx, &mq.GithubRepoRequestMessage{
			Owner: owner,
			Repo:  repo,
		})
		if err != nil {
			return nil, err
		}

		return source, nil
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, source)
}
