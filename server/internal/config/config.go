// Package config loads the server configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Store backends.
const (
	StoreGitHub = "github"
	StoreLocal  = "local"
)

// Config is the complete runtime configuration. See README.md for the
// meaning of every environment variable.
type Config struct {
	Port string

	GitHubToken  string
	GitHubAPIURL string

	DefaultRepo  string
	AllowedRepos []string

	AssetsRepo   string // empty: assets live in the issue's repo
	AssetsBranch string

	AllowedOrigins []string // "*" allows any origin
	APIKeys        []string // empty: no auth

	Labels        []string // always applied
	AllowedLabels []string // client labels that may pass through; empty: drop all

	MaxUploadBytes int64 // total of all attachments
	MaxFileBytes   int64 // per attachment

	RateLimit  int // requests per minute per client IP; 0 disables
	TrustProxy bool

	Store    string
	LocalDir string
	DryRun   bool

	PublicURL string // base URL of this service, used for local assets and dry-run previews

	// Azure OpenAI for the "write it for me" assistant (POST /v1/assist).
	// Assist is enabled only when endpoint, API key and deployment are all set.
	AzureOpenAIEndpoint        string
	AzureOpenAIAPIKey          string
	AzureOpenAIDeployment      string
	AzureOpenAIReasoningEffort string
}

// AssistEnabled reports whether the Azure OpenAI assistant is configured.
func (c *Config) AssistEnabled() bool {
	return c.AzureOpenAIEndpoint != "" && c.AzureOpenAIAPIKey != "" && c.AzureOpenAIDeployment != ""
}

// Load reads the configuration using getenv (typically os.Getenv).
func Load(getenv func(string) string) (*Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	c := &Config{
		Port:          get("PORT", "8080"),
		GitHubToken:   strings.TrimSpace(getenv("GITHUB_TOKEN")),
		GitHubAPIURL:  strings.TrimRight(get("GITHUB_API_URL", "https://api.github.com"), "/"),
		DefaultRepo:   get("INSPECT_DEFAULT_REPO", ""),
		AssetsRepo:    get("INSPECT_ASSETS_REPO", ""),
		AssetsBranch:  get("INSPECT_ASSETS_BRANCH", "inspect-assets"),
		Store:         strings.ToLower(get("INSPECT_STORE", StoreGitHub)),
		LocalDir:      get("INSPECT_LOCAL_DIR", "./data"),
		AllowedLabels: splitList(getenv("INSPECT_ALLOWED_LABELS")),
		APIKeys:       splitList(getenv("INSPECT_API_KEYS")),

		AzureOpenAIEndpoint:        strings.TrimRight(strings.TrimSpace(getenv("AZURE_OPENAI_ENDPOINT")), "/"),
		AzureOpenAIAPIKey:          strings.TrimSpace(getenv("AZURE_OPENAI_API_KEY")),
		AzureOpenAIDeployment:      strings.TrimSpace(getenv("AZURE_OPENAI_DEPLOYMENT")),
		AzureOpenAIReasoningEffort: strings.TrimSpace(getenv("AZURE_OPENAI_REASONING_EFFORT")),
	}

	var errs []error

	if c.DefaultRepo == "" {
		errs = append(errs, errors.New("INSPECT_DEFAULT_REPO is required"))
	} else if !ValidRepo(c.DefaultRepo) {
		errs = append(errs, fmt.Errorf("INSPECT_DEFAULT_REPO %q is not owner/name", c.DefaultRepo))
	}

	c.AllowedRepos = splitList(getenv("INSPECT_ALLOWED_REPOS"))
	if len(c.AllowedRepos) == 0 && c.DefaultRepo != "" {
		c.AllowedRepos = []string{c.DefaultRepo}
	}
	for _, r := range c.AllowedRepos {
		if !ValidRepo(r) {
			errs = append(errs, fmt.Errorf("INSPECT_ALLOWED_REPOS entry %q is not owner/name", r))
		}
	}
	if c.DefaultRepo != "" && !containsFold(c.AllowedRepos, c.DefaultRepo) {
		c.AllowedRepos = append(c.AllowedRepos, c.DefaultRepo)
	}
	if c.AssetsRepo != "" && !ValidRepo(c.AssetsRepo) {
		errs = append(errs, fmt.Errorf("INSPECT_ASSETS_REPO %q is not owner/name", c.AssetsRepo))
	}

	c.AllowedOrigins = splitList(getenv("INSPECT_ALLOWED_ORIGINS"))
	for i, o := range c.AllowedOrigins {
		c.AllowedOrigins[i] = strings.TrimRight(o, "/")
	}

	if v := getenv("INSPECT_LABELS"); strings.TrimSpace(v) != "" {
		c.Labels = splitList(v)
	} else {
		c.Labels = []string{"inspect"}
	}

	var err error
	if c.DryRun, err = parseBool(getenv("INSPECT_DRY_RUN")); err != nil {
		errs = append(errs, fmt.Errorf("INSPECT_DRY_RUN: %w", err))
	}
	if c.TrustProxy, err = parseBool(getenv("INSPECT_TRUST_PROXY")); err != nil {
		errs = append(errs, fmt.Errorf("INSPECT_TRUST_PROXY: %w", err))
	}

	totalMB, err := parsePositiveInt(get("INSPECT_MAX_UPLOAD_MB", "50"))
	if err != nil {
		errs = append(errs, fmt.Errorf("INSPECT_MAX_UPLOAD_MB: %w", err))
	}
	fileMB, err := parsePositiveInt(get("INSPECT_MAX_FILE_MB", strconv.Itoa(totalMB)))
	if err != nil {
		errs = append(errs, fmt.Errorf("INSPECT_MAX_FILE_MB: %w", err))
	}
	if fileMB > totalMB {
		fileMB = totalMB
	}
	c.MaxUploadBytes = int64(totalMB) << 20
	c.MaxFileBytes = int64(fileMB) << 20

	rl, err := strconv.Atoi(get("INSPECT_RATE_LIMIT", "10"))
	if err != nil || rl < 0 {
		errs = append(errs, fmt.Errorf("INSPECT_RATE_LIMIT: must be a non-negative integer"))
	}
	c.RateLimit = rl

	switch c.Store {
	case StoreGitHub, StoreLocal:
	default:
		errs = append(errs, fmt.Errorf("INSPECT_STORE %q: must be github or local", c.Store))
	}

	if c.GitHubToken == "" && !(c.Store == StoreLocal && c.DryRun) {
		errs = append(errs, errors.New("GITHUB_TOKEN is required (unless INSPECT_STORE=local and INSPECT_DRY_RUN=true)"))
	}

	c.PublicURL = strings.TrimRight(get("INSPECT_PUBLIC_URL", "http://localhost:"+c.Port), "/")

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// RepoAllowed reports whether repo is in the allowlist (case-insensitive,
// as GitHub owner/repo names are).
func (c *Config) RepoAllowed(repo string) bool { return containsFold(c.AllowedRepos, repo) }

// OriginAllowed reports whether a browser Origin may call the API.
func (c *Config) OriginAllowed(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

// ValidRepo reports whether s looks like "owner/name".
func ValidRepo(s string) bool {
	owner, name, ok := strings.Cut(s, "/")
	return ok && validRepoPart(owner) && validRepoPart(name)
}

func validRepoPart(s string) bool {
	if s == "" || len(s) > 100 || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func parseBool(s string) (bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return false, nil
	}
	return strconv.ParseBool(s)
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return n, nil
}
