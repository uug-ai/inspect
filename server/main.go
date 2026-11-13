// Command inspect-server receives bug reports from the inspect browser
// library, stores their media and files them as GitHub issues.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/uug-ai/inspect/server/internal/assist"
	"github.com/uug-ai/inspect/server/internal/config"
	"github.com/uug-ai/inspect/server/internal/github"
	"github.com/uug-ai/inspect/server/internal/httpapi"
	"github.com/uug-ai/inspect/server/internal/issue"
	"github.com/uug-ai/inspect/server/internal/site"
	"github.com/uug-ai/inspect/server/internal/store"
)

func main() {
	if runFlags(os.Args[1:]) {
		return
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	// Local development: .env (committed defaults) then .env.local (your token, gitignored).
	loaded, err := loadEnvFiles(".env", ".env.local")
	if err != nil {
		log.Error("invalid env file", "err", err)
		os.Exit(1)
	}
	if len(loaded) > 0 {
		log.Info("loaded env files", "files", loaded)
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           build(cfg, log),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      4 * time.Minute, // includes uploading media to GitHub
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", srv.Addr, "store", cfg.Store, "dry_run", cfg.DryRun,
			"default_repo", cfg.DefaultRepo, "auth", len(cfg.APIKeys) > 0)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

// build wires the configuration into an http.Handler.
func build(cfg *config.Config, log *slog.Logger) http.Handler {
	if len(cfg.AllowedOrigins) == 0 {
		log.Warn("INSPECT_ALLOWED_ORIGINS is empty: browser requests (with an Origin header) will be rejected")
	}

	gh := github.NewClient(cfg.GitHubAPIURL, github.StaticToken(cfg.GitHubToken))

	var local *store.Local
	var storeFor func(string) store.Store
	switch cfg.Store {
	case config.StoreLocal:
		local = &store.Local{Dir: cfg.LocalDir, BaseURL: cfg.PublicURL}
		storeFor = func(string) store.Store { return local }
	default:
		var mu sync.Mutex
		stores := map[string]*store.GitHub{}
		storeFor = func(issueRepo string) store.Store {
			repo := cfg.AssetsRepo
			if repo == "" {
				repo = issueRepo
			}
			mu.Lock()
			defer mu.Unlock()
			key := strings.ToLower(repo)
			st, ok := stores[key]
			if !ok {
				st = store.NewGitHub(gh, repo, cfg.AssetsBranch)
				stores[key] = st
			}
			return st
		}
	}

	var previews *issue.Previews
	if cfg.DryRun {
		previews = issue.NewPreviews(100)
	}

	return (&httpapi.Server{
		Cfg: cfg,
		Issues: &issue.Service{
			Cfg:      cfg,
			Issues:   gh,
			StoreFor: storeFor,
			Previews: previews,
			Log:      log,
		},
		Local:    local,
		Previews: previews,
		Limiter:  httpapi.NewRateLimiter(cfg.RateLimit),
		Log:      log,
		Site:     site.Register,

		Assist:        assistant(cfg, log),
		AssistLimiter: httpapi.NewRateLimiter(cfg.RateLimit),
	}).Handler()
}

// assistant returns the Azure OpenAI writing assistant, or nil when it isn't configured.
func assistant(cfg *config.Config, log *slog.Logger) httpapi.Assistant {
	if !cfg.AssistEnabled() {
		log.Info("writing assistant disabled (set AZURE_OPENAI_ENDPOINT, AZURE_OPENAI_API_KEY and AZURE_OPENAI_DEPLOYMENT to enable)")
		return nil
	}
	c, err := assist.New(assist.Config{
		Endpoint:        cfg.AzureOpenAIEndpoint,
		APIKey:          cfg.AzureOpenAIAPIKey,
		Deployment:      cfg.AzureOpenAIDeployment,
		ReasoningEffort: cfg.AzureOpenAIReasoningEffort,
	})
	if err != nil {
		log.Error("writing assistant disabled", "err", err)
		return nil
	}
	log.Info("writing assistant enabled", "deployment", cfg.AzureOpenAIDeployment)
	return c
}
