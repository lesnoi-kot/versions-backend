package common_test

import (
	"testing"

	"github.com/lesnoi-kot/versions-backend/common"
)

func TestParseGithubLink(t *testing.T) {
	type testCase struct {
		link  string
		owner string
		repo  string
		err   bool
	}

	testCases := []testCase{
		{"https://github.com/lesnoi-kot/karten-backend", "lesnoi-kot", "karten-backend", false},
		{"https://github.com/lesnoi-kot/karten-backend/", "lesnoi-kot", "karten-backend", false},
		{"https://github.com/lesnoi-kot/karten-backend/bla-bla", "lesnoi-kot", "karten-backend", false},
		{"https://github.com/a/b", "a", "b", false},
		{"https://github.com/lesnoi-kot", "", "", true},
		{"xxxxx", "", "", true},
		{"https://github.com", "", "", true},
	}

	for _, test := range testCases {
		t.Run(test.link, func(t *testing.T) {
			owner, repo, err := common.ParseGithubRepoLink(test.link)

			if test.err && err == nil {
				t.Error("Expected error")
			} else if test.owner != owner || test.repo != repo {
				t.Errorf("Owner and repo did not match: %s != %s, %s != %s", owner, test.owner, repo, test.repo)
			}
		})

	}
}
