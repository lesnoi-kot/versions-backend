package dataloader

import "github.com/shurcooL/githubv4"

type queryBriefRepo struct {
	Repository struct {
		ID         githubv4.ID
		DatabaseId githubv4.Int

		Releases struct {
			TotalCount githubv4.Int
		}

		Refs struct {
			TotalCount githubv4.Int
		} `graphql:"refs(refPrefix: \"refs/tags/\")"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type queryReleases struct {
	RateLimit struct {
		Remaining githubv4.Int
	}

	Repository struct {
		ID githubv4.ID

		Releases struct {
			TotalCount githubv4.Int

			PageInfo struct {
				EndCursor   githubv4.String
				StartCursor githubv4.String
			}

			Nodes []struct {
				ID          githubv4.ID
				Name        githubv4.String
				TagName     githubv4.String
				PublishedAt githubv4.DateTime
				URL         githubv4.String
			}
		} `graphql:"releases(after: $afterRelease, first: $perPage, orderBy: $order)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type queryTags struct {
	RateLimit struct {
		Remaining githubv4.Int
	}

	Repository struct {
		ID githubv4.ID

		Refs struct {
			TotalCount githubv4.Int

			PageInfo struct {
				EndCursor   githubv4.String
				StartCursor githubv4.String
			}

			Nodes []struct {
				ID   githubv4.ID
				Name githubv4.String

				Tag struct {
					CommitURL githubv4.String

					Tagger struct {
						Date githubv4.DateTime
					}
				} `graphql:"... on Tag"`
			}
		} `graphql:"refs(after: $afterRelease, first: $perPage, orderBy: $order, refPrefix: \"refs/tags/\")"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}
