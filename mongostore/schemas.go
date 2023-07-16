package mongostore

import (
	"time"

	"github.com/Masterminds/semver/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Source struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt,omitempty"`
	ExternalID  string             `bson:"external_id" json:"-"`
	Owner       string             `bson:"owner" json:"owner,omitempty"`
	Name        string             `bson:"name" json:"name,omitempty"`
	Description string             `bson:"description" json:"description,omitempty"`
	URL         string             `bson:"url" json:"url,omitempty"`
	Releases    []Release          `bson:"releases" json:"releases,omitempty"`
	IsFetching  bool               `bson:"is_fetching" json:"isFetching"`
	EndCursor   *string            `bson:"end_cursor" json:"-"`
}

type Release struct {
	ID           string    `bson:"id" json:"id"`
	Name         string    `bson:"name" json:"name"`
	TagName      string    `bson:"tag_name" json:"tagName"`
	URL          string    `bson:"url" json:"url"`
	PublishedAt  time.Time `bson:"published_at" json:"publishedAt"`
	IsSemver     bool      `bson:"is_semver" json:"isSemver"`
	Major        uint64    `bson:"major" json:"major"`
	Minor        uint64    `bson:"minor" json:"minor"`
	Patch        uint64    `bson:"patch" json:"patch"`
	IsPrerelease bool      `bson:"is_prerelease" json:"isPrerelease"`
}

func (r *Release) ParseTagName() {
	version, err := semver.NewVersion(r.TagName)
	if err != nil {
		return
	}

	r.IsSemver = true
	r.Major = version.Major()
	r.Minor = version.Minor()
	r.Patch = version.Patch()
	r.IsPrerelease = version.Prerelease() != ""
}
