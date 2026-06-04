package awsresources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/ini.v1"
)

// configCache memoizes loaded aws.Config values keyed by "profile|region" so
// repeated tool calls within a session avoid re-resolving SSO/shared config.
type configCache struct {
	mu sync.Mutex
	m  map[string]aws.Config
}

// newConfigCache returns an empty config cache.
func newConfigCache() *configCache { return &configCache{m: map[string]aws.Config{}} }

// get returns a cached aws.Config for profile|region, loading via the default
// credential chain (env/SSO/shared config) on first use. An empty profile or
// region falls back to the SDK's own defaults.
func (c *configCache) get(ctx context.Context, profile, region string) (aws.Config, error) {
	key := profile + "|" + region
	c.mu.Lock()
	defer c.mu.Unlock()
	if cfg, ok := c.m[key]; ok {
		return cfg, nil
	}
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config for profile %q: %w (try: aws sso login --profile %s)", profile, err, profile)
	}
	c.m[key] = cfg
	return cfg, nil
}

// defaultConfigPath returns the shared AWS config file path, honoring the
// AWS_CONFIG_FILE environment variable when set.
func defaultConfigPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "config")
}

// listProfilesFrom parses an ~/.aws/config file and returns the profile names.
// Sections are either "[default]" or "[profile NAME]"; the ini DEFAULT section
// is ignored. The file is read solely to enumerate names — no values are
// returned.
func listProfilesFrom(path string) ([]string, error) {
	f, err := ini.Load(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range f.Sections() {
		name := s.Name()
		switch {
		case name == ini.DefaultSection:
			continue
		case name == "default":
			out = append(out, "default")
		case strings.HasPrefix(name, "profile "):
			out = append(out, strings.TrimPrefix(name, "profile "))
		}
	}
	return out, nil
}
