package githubaccount

import (
	"context"
	"net/url"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// newClient builds a go-github client authenticated with token. If baseURL is
// non-empty it overrides the API endpoint (used by tests to target httptest).
func newClient(token, baseURL string) *github.Client {
	httpClient := oauth2.NewClient(context.Background(),
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	gh := github.NewClient(httpClient)
	if baseURL != "" {
		u, _ := url.Parse(baseURL)
		gh.BaseURL = u
	}
	return gh
}
