package githubaccount

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientUsesBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()

	gh := newClient("test-token", srv.URL+"/")
	if gh.BaseURL.String() != srv.URL+"/" {
		t.Fatalf("BaseURL = %q, want %q", gh.BaseURL.String(), srv.URL+"/")
	}
}

func TestNewServerRequiresToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	if _, err := NewServer(); err == nil {
		t.Fatal("expected error when GITHUB_TOKEN is unset")
	}
}
