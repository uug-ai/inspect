package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/uug-ai/inspect/server/internal/config"
)

// runFlags handles the offline self-test entrypoints used by the hardened
// image smoke test. It returns false when there are no arguments, meaning
// "start the server".
func runFlags(args []string) bool {
	if len(args) == 0 {
		return false
	}
	var err error
	switch {
	case len(args) == 1 && args[0] == "--self-test":
		err = selfTest()
	case len(args) == 2 && args[0] == "--self-test-dns":
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = selfTestDNS(ctx, args[1])
	default:
		fmt.Fprintln(os.Stderr, "usage: main [--self-test | --self-test-dns HOSTNAME]  (no arguments starts the server)")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "self-test failed:", err)
		os.Exit(1)
	}
	fmt.Println("runtime self-test passed")
	return true
}

// selfTest checks the static runtime without contacting GitHub: CA
// certificates are present, and the HTTP handler serves /healthz over loopback.
func selfTest() error {
	if _, err := os.ReadFile("/etc/resolv.conf"); err != nil {
		return fmt.Errorf("read DNS resolver configuration: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("load system certificates: %w", err)
	}
	if len(roots.Subjects()) == 0 { //nolint:staticcheck // only checking the pool is non-empty
		return fmt.Errorf("system certificate pool is empty")
	}

	// A dry-run local configuration: nothing is written unless a report is posted.
	cfg, err := config.Load(func(key string) string {
		return map[string]string{
			"INSPECT_DRY_RUN":      "true",
			"INSPECT_STORE":        "local",
			"INSPECT_DEFAULT_REPO": "uug-ai/inspect",
			"INSPECT_LOCAL_DIR":    os.TempDir(),
		}[key]
	})
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on loopback: %w", err)
	}
	srv := &http.Server{
		Handler:           build(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go srv.Serve(ln) //nolint:errcheck // closed below
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		return fmt.Errorf("GET /healthz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /healthz: status %d", resp.StatusCode)
	}
	return nil
}

func selfTestDNS(ctx context.Context, hostname string) error {
	if hostname == "" {
		return fmt.Errorf("DNS self-test requires a hostname")
	}
	if err := selfTest(); err != nil {
		return err
	}
	addresses, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", hostname, err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("resolve %s: no addresses", hostname)
	}
	return nil
}
