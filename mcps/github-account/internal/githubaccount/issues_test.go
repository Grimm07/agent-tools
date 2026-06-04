package githubaccount

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestListIssuesFiltersPullRequests(t *testing.T) {
	var gotQuery string
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/issues" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[
			{"number":1,"title":"bug","state":"open","html_url":"u1",
			 "user":{"login":"ada"},"labels":[{"name":"bug"}]},
			{"number":2,"title":"a pr","state":"open","html_url":"u2",
			 "user":{"login":"bob"},"pull_request":{"url":"x"}}
		]`))
	})
	defer done()

	_, out, err := ListIssues(context.Background(), gh, ListIssuesInput{
		Owner: "me", Repo: "app", State: "open", Labels: []string{"bug"}, Assignee: "ada",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Issues) != 1 {
		t.Fatalf("expected 1 issue (PR filtered), got %+v", out.Issues)
	}
	is := out.Issues[0]
	if is.Number != 1 || is.Title != "bug" || is.Author != "ada" ||
		len(is.Labels) != 1 || is.Labels[0] != "bug" {
		t.Fatalf("projection wrong: %+v", is)
	}
	if !strings.Contains(gotQuery, "state=open") || !strings.Contains(gotQuery, "labels=bug") ||
		!strings.Contains(gotQuery, "assignee=ada") {
		t.Errorf("query missing filters: %q", gotQuery)
	}
}

func TestGetIssue(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/issues/7" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"number":7,"title":"t","state":"closed","body":"b",
			"html_url":"u","user":{"login":"ada"},"labels":[{"name":"x"}]}`))
	})
	defer done()

	_, out, err := GetIssue(context.Background(), gh, GetIssueInput{Owner: "me", Repo: "app", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if out.Number != 7 || out.Body != "b" || out.State != "closed" || out.Author != "ada" {
		t.Fatalf("got %+v", out)
	}
}

func TestListIssueComments(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/issues/7/comments" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`[{"body":"hi","html_url":"u","user":{"login":"ada"},
			"created_at":"2021-05-01T12:00:00Z"}]`))
	})
	defer done()

	_, out, err := ListIssueComments(context.Background(), gh, ListIssueCommentsInput{
		Owner: "me", Repo: "app", Number: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Comments) != 1 || out.Comments[0].Author != "ada" || out.Comments[0].Body != "hi" {
		t.Fatalf("got %+v", out.Comments)
	}
	if !strings.Contains(out.Comments[0].CreatedAt, "2021-05-01") {
		t.Errorf("created_at not mapped: %q", out.Comments[0].CreatedAt)
	}
}
