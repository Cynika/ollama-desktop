package util

import (
	"net/http"
	"testing"
)

func TestGiteeRelease_Last(t *testing.T) {
	release := GiteeRelease{
		Http: http.DefaultClient,
	}
	item, err := release.Last("jianggujin", "ollama-desktop")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v\n", item)
}

func TestGithubRelease_Last(t *testing.T) {
	release := GithubRelease{
		Http: http.DefaultClient,
	}
	item, err := release.Last("jianggujin", "ollama-desktop")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v\n", item)
}
