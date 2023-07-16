package common

import (
	"errors"
	"regexp"
)

var repoRegexp = regexp.MustCompile(`^https://github\.com/([a-zA-Z0-9-_]+)/([a-zA-Z0-9-_]+)/?`)

func ParseGithubRepoLink(link string) (string, string, error) {
	matches := repoRegexp.FindStringSubmatch(link)
	if len(matches) != 3 {
		return "", "", errors.New("invalid github repo link")
	}

	owner, repo := matches[1], matches[2]
	return owner, repo, nil
}
