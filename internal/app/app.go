package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/lucasew/gaderno/internal/config"
	"github.com/lucasew/gaderno/internal/log"
	"github.com/lucasew/gaderno/internal/store"
	"github.com/lucasew/gaderno/internal/workspace"
)

// Run starts the HTTP server until ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, version string) error {
	logger := log.New()

	root, err := cfg.AbsRoot()
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	ws := workspace.New(root)
	st := store.New(root)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, version)
	})
	registerWorkspaceRoutes(mux, ws, logger)
	registerNotebookRoutes(mux, st, logger)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           withLogging(logger, mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	logger.Info("listening", "addr", ln.Addr().String(), "root", root, "version", version)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
