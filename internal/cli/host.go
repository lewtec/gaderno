package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lewtec/lewkit/x/driver"
	"github.com/lewtec/lewkit/x/driver/webview"
	"github.com/lewtec/lewkit/x/entry"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lewtec/lewkit/x/taskgroup"
	"github.com/lucasew/gaderno/internal/app"
	"github.com/lucasew/gaderno/internal/auth"
	"github.com/lucasew/gaderno/internal/config"
)

// HostLaunch reports whether this process should start the packaged app
// instead of the CLI. A host sets ELETROCROMO_NO_UI. lewkit release run
// starts the binary with an empty argument list.
func HostLaunch(args []string) bool {
	if hostNoUI() {
		return true
	}
	return len(args) == 0
}

// RunHost serves the notebook UI for a lewkit app process.
// The folder window picks the directory first.
// entry.Main calls it on the UI thread. The Android host calls it
// through entry.RunBound, which has not bound that thread yet.
func RunHost(ctx context.Context) error {
	if taskgroup.FromContext(ctx) != nil {
		return runHost(ctx)
	}
	return entry.Run(ctx, runHost)
}

func runHost(ctx context.Context) error {
	root, err := appDir(ctx)
	if err != nil {
		return err
	}
	if root == "" {
		return nil
	}
	cfg, err := hostConfig(root)
	if err != nil {
		return err
	}
	if err := auth.CheckBind(cfg.Listen, cfg.Token, false); err != nil {
		return err
	}
	if hostNoUI() {
		return serveReady(ctx, cfg)
	}
	return openWindow(ctx, cfg, true)
}

func hostConfig(root string) (config.Config, error) {
	kernel := strings.TrimSpace(os.Getenv("GADERNO_KERNEL"))
	if kernel == "" {
		kernel = "python3"
	}
	return config.Config{
		Root:   root,
		Listen: hostListen(),
		Token:  strings.TrimSpace(os.Getenv("GADERNO_TOKEN")),
		Kernel: kernel,
	}, nil
}

func hostListen() string {
	v := strings.TrimSpace(os.Getenv("GADERNO_LISTEN"))
	if v == "" {
		return "127.0.0.1:0"
	}
	if !strings.Contains(v, ":") {
		return "127.0.0.1:" + v
	}
	return v
}

func hostRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GADERNO_ROOT")); v != "" {
		return v, nil
	}
	if hostNoUI() {
		return packagedNotebooks()
	}
	return ".", nil
}

func packagedNotebooks() (string, error) {
	base := strings.TrimSpace(os.Getenv("LEWKIT_DATA_DIR"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("ELETROCROMO_DATA_DIR"))
	}
	if base == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("app data: %w", err)
		}
		base = filepath.Join(dir, "gaderno")
	}
	dir := filepath.Join(base, "notebooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("notebooks: %w", err)
	}
	return dir, nil
}

func hostNoUI() bool {
	return envOn("ELETROCROMO_NO_UI") || envOn("LEWKIT_NO_UI")
}

// readyLinePrefix is the line packaged hosts parse on stdout.
// Keep it identical to lewkit x/app.ReadyLinePrefix.
const readyLinePrefix = "ELETROCROMO_READY "

func envOn(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

// serveReady is the packaged host path. The host web view loads the URL
// after ELETROCROMO_READY. The server is real HTTP so the notebook socket works.
func serveReady(ctx context.Context, cfg config.Config) error {
	return app.RunReady(ctx, cfg, release.Version(), func(addr string) {
		announceReady(addr, cfg.Token)
	})
}

func announceReady(addr, token string) {
	target, err := desktopURL(addr, token)
	if err != nil {
		slog.Error("ready", "err", err)
		return
	}
	fmt.Fprintln(os.Stdout, readyLinePrefix+target)
	_ = os.Stdout.Sync()
	entry.NotifyReady(target)
	path := strings.TrimSpace(os.Getenv("ELETROCROMO_READY_FILE"))
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(target+"\n"), 0o600); err != nil {
		slog.Error("ready file", "err", err)
	}
}

// openWindow serves on loopback, then points a web view at that URL.
// host reports ELETROCROMO_READY and keeps serving when no web view driver exists.
func openWindow(ctx context.Context, cfg config.Config, host bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.RunReady(ctx, cfg, release.Version(), func(addr string) {
			if host {
				announceReady(addr, cfg.Token)
			}
			ready <- addr
		})
	}()

	var addr string
	select {
	case <-ctx.Done():
		if host {
			return nil
		}
		return context.Cause(ctx)
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("desktop: %w", err)
		}
		return nil
	case addr = <-ready:
	}

	page, err := desktopPage(addr, cfg.Token)
	if err != nil {
		return err
	}
	view, err := webview.Open(ctx, webview.Config{
		Title:  "gaderno",
		Width:  1200,
		Height: 800,
		HTML:   page,
	})
	if err != nil {
		if host && errors.Is(err, driver.ErrUnavailable) {
			select {
			case <-ctx.Done():
				<-errCh
				return nil
			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("desktop: %w", err)
				}
				return nil
			}
		}
		return fmt.Errorf("window: %w", err)
	}

	select {
	case <-ctx.Done():
		closeErr := view.Close()
		return errors.Join(<-errCh, closeErr)
	case <-view.Done():
		cancel()
		return <-errCh
	case err := <-errCh:
		closeErr := view.Close()
		if err != nil {
			err = fmt.Errorf("desktop: %w", err)
		}
		return errors.Join(err, closeErr)
	}
}
