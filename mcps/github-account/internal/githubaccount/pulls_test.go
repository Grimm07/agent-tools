package githubaccount

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestListPullRequests(t *testing.T) {
	var gotQuery string
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/pulls" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[{"number":3,"title":"feat","state":"open","html_url":"u",
			"user":{"login":"ada"},
			"base":{"ref":"main"},"head":{"ref":"topic"}}]`))
	})
	defer done()

	_, out, err := ListPullRequests(context.Background(), gh, ListPullRequestsInput{
		Owner: "me", Repo: "app", State: "open", Base: "main", Head: "ada:topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.PullRequests) != 1 {
		t.Fatalf("got %+v", out.PullRequests)
	}
	p := out.PullRequests[0]
	if p.Number != 3 || p.Base != "main" || p.Head != "topic" || p.Author != "ada" {
		t.Fatalf("projection wrong: %+v", p)
	}
	if !strings.Contains(gotQuery, "state=open") || !strings.Contains(gotQuery, "base=main") {
		t.Errorf("query missing filters: %q", gotQuery)
	}
}

func TestGetPullRequest(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/pulls/3" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"number":3,"title":"feat","state":"closed","body":"b",
			"merged":true,"html_url":"u",
			"base":{"ref":"main"},"head":{"ref":"topic"}}`))
	})
	defer done()

	_, out, err := GetPullRequest(context.Background(), gh, GetPullRequestInput{Owner: "me", Repo: "app", Number: 3})
	if err != nil {
		t.Fatal(err)
	}
	if out.Number != 3 || out.Body != "b" || !out.Merged ||
		out.Base != "main" || out.Head != "topic" {
		t.Fatalf("got %+v", out)
	}
}

func TestGetPullRequestDiff(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/pulls/3/files" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`[{"filename":"a.go","status":"modified","additions":2,
			"deletions":1,"patch":"@@ -1 +1 @@"}]`))
	})
	defer done()

	_, out, err := GetPullRequestDiff(context.Background(), gh, GetPullRequestDiffInput{Owner: "me", Repo: "app", Number: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 1 {
		t.Fatalf("got %+v", out.Files)
	}
	f := out.Files[0]
	if f.Filename != "a.go" || f.Status != "modified" || f.Additions != 2 ||
		f.Deletions != 1 || !strings.Contains(f.Patch, "@@") {
		t.Fatalf("projection wrong: %+v", f)
	}
}
