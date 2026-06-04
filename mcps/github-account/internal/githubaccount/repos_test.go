package githubaccount

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

// newTestClientFromHandler spins up an httptest server with the given handler and
// returns a go-github client pointed at it plus a cleanup func.
func newTestClientFromHandler(t *testing.T, h http.HandlerFunc) (*github.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	gh := newClient("t", srv.URL+"/")
	return gh, srv.Close
}

func TestClampPerPage(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, defaultPerPage}, {-5, defaultPerPage}, {10, 10}, {100, 100}, {500, maxPerPage},
	}
	for _, c := range cases {
		if got := clampPerPage(c.in); got != c.want {
			t.Errorf("clampPerPage(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestListRepos(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Link", `<https://x/user/repos?page=2>; rel="next"`)
		w.Write([]byte(`[{"name":"app","full_name":"me/app","private":true,
			"description":"d","default_branch":"main","html_url":"u"}]`))
	})
	defer done()

	_, out, err := ListRepos(context.Background(), gh, ListReposInput{PerPage: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Repos) != 1 || out.Repos[0].FullName != "me/app" {
		t.Fatalf("got %+v", out.Repos)
	}
	if !out.Repos[0].Private || out.Repos[0].DefaultBranch != "main" {
		t.Fatalf("projection wrong: %+v", out.Repos[0])
	}
	if !out.HasMore {
		t.Errorf("expected HasMore=true from Link header")
	}
}

func TestListReposError(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	})
	defer done()

	_, _, err := ListRepos(context.Background(), gh, ListReposInput{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention status 403: %v", err)
	}
}

func TestGetRepo(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"name":"app","full_name":"me/app","private":false,"default_branch":"main","html_url":"u"}`))
	})
	defer done()

	_, out, err := GetRepo(context.Background(), gh, GetRepoInput{Owner: "me", Repo: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Repo.FullName != "me/app" {
		t.Fatalf("got %+v", out.Repo)
	}
}

func TestGetRepoNotFound(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})
	defer done()

	_, _, err := GetRepo(context.Background(), gh, GetRepoInput{Owner: "me", Repo: "nope"})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
}

func TestListBranches(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/branches" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`[{"name":"main","commit":{"sha":"abc"},"protected":true}]`))
	})
	defer done()

	_, out, err := ListBranches(context.Background(), gh, ListBranchesInput{Owner: "me", Repo: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Branches) != 1 || out.Branches[0].Name != "main" ||
		out.Branches[0].CommitSHA != "abc" || !out.Branches[0].Protected {
		t.Fatalf("got %+v", out.Branches)
	}
}

func TestListCommits(t *testing.T) {
	var gotQuery string
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/commits" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[{"sha":"abc","html_url":"u","commit":{"message":"m",
			"author":{"name":"Ada","date":"2020-01-01T00:00:00Z"}}}]`))
	})
	defer done()

	_, out, err := ListCommits(context.Background(), gh, ListCommitsInput{
		Owner: "me", Repo: "app", Ref: "main", Path: "x", PerPage: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Commits) != 1 || out.Commits[0].SHA != "abc" ||
		out.Commits[0].Message != "m" || out.Commits[0].Author != "Ada" {
		t.Fatalf("got %+v", out.Commits)
	}
	if !strings.Contains(out.Commits[0].Date, "2020-01-01") {
		t.Errorf("date not mapped: %q", out.Commits[0].Date)
	}
	if !strings.Contains(gotQuery, "sha=main") || !strings.Contains(gotQuery, "path=x") ||
		!strings.Contains(gotQuery, "per_page=5") {
		t.Errorf("query missing params: %q", gotQuery)
	}
}

func TestGetFile(t *testing.T) {
	// "hello\n" base64-encoded.
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"file","encoding":"base64","path":"README.md",
			"size":6,"content":"aGVsbG8K"}`))
	})
	defer done()

	_, out, err := GetFile(context.Background(), gh, GetFileInput{Owner: "me", Repo: "app", Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Type != "file" || out.Content != "hello\n" || out.Size != 6 {
		t.Fatalf("got %+v", out)
	}
}

func TestGetFileDirectory(t *testing.T) {
	gh, done := newTestClientFromHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"type":"file","path":"a.go","size":10},
			{"type":"dir","path":"sub","size":0}]`))
	})
	defer done()

	_, out, err := GetFile(context.Background(), gh, GetFileInput{Owner: "me", Repo: "app", Path: "."})
	if err != nil {
		t.Fatal(err)
	}
	if out.Type != "dir" || len(out.Entries) != 2 || out.Entries[0].Path != "a.go" {
		t.Fatalf("got %+v", out)
	}
}
