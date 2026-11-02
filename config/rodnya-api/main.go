// rodnya-api keeps what visitors of the private pages under /rodnya/<page>/ select and write there.
// Each change is posted with a random id the browser keeps; the page reads its state back on load,
// and the owner reads everything under /rodnya/<page>/api/admin/<key>/.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/go-pkgz/lgr"
	"github.com/jessevdk/go-flags"
)

type options struct {
	Listen   string `long:"listen" env:"LISTEN" default:":8080" description:"listen address"`
	Pages    string `long:"pages" env:"PAGES" default:"/srv/pages" description:"directory with one folder per published page"`
	Data     string `long:"data" env:"DATA" default:"/srv/var" description:"directory for the per-page logs"`
	AdminKey string `long:"admin-key" env:"ADMIN_KEY" description:"32 hex characters opening the summary; empty disables it"`
	Check    bool   `long:"check" description:"query /ping on the listen address and exit, for the container healthcheck"`
	Dbg      bool   `long:"dbg" env:"DEBUG" description:"debug logging"`
}

func main() {
	var opts options
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}
	if opts.Check {
		os.Exit(healthcheck(opts.Listen))
	}
	if opts.Dbg {
		log.Setup(log.Debug)
	}
	if opts.AdminKey != "" && !adminKeyRe.MatchString(opts.AdminKey) {
		log.Fatalf("[ERROR] admin key must be 32 lowercase hex characters")
	}
	if err := os.MkdirAll(opts.Data, 0o700); err != nil {
		log.Fatalf("[ERROR] data dir: %v", err)
	}

	st := newStore(opts.Data)
	defer st.close()
	srv := &server{store: st, pages: opts.Pages, adminKey: opts.AdminKey, limiter: newLimiter(5, 40)}
	httpSrv := &http.Server{
		Addr:              opts.Listen,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()
	go srv.limiter.cleanupLoop(ctx, 10*time.Minute)

	log.Printf("[INFO] listening on %s, pages %s, data %s, summary enabled: %v", opts.Listen, opts.Pages, opts.Data, opts.AdminKey != "")
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[ERROR] server: %v", err)
	}
}

func healthcheck(listen string) int {
	host := listen
	if host != "" && host[0] == ':' {
		host = "127.0.0.1" + host
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/ping")
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
