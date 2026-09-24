package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lewtec/lewkit/x/driver/webview"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lewtec/lewkit/x/thread"
	"github.com/lucasew/gaderno/internal/app"
	"github.com/lucasew/gaderno/internal/auth"
	"github.com/lucasew/gaderno/internal/config"

	_ "github.com/lewtec/lewkit/x/driver/webview/prelude"
)

type desktopCmd struct {
	Root   cmd.WorkDirArg `short:"C" long:"root" env:"GADERNO_ROOT" help:"workspace root directory"`
	Listen cmd.AddrArg    `long:"listen" env:"GADERNO_LISTEN,PORT" default:"127.0.0.1:0" help:"loopback listen address"`
	Token  cmd.StringArg  `long:"token" env:"GADERNO_TOKEN" default:"" help:"shared access token"`
	Kernel cmd.StringArg  `long:"kernel" env:"GADERNO_KERNEL" default:"python3" help:"default kernelspec name"`
}

func (desktopCmd) Description() string {
	return "open the notebook UI in a desktop window"
}

func (c *desktopCmd) Run(ctx context.Context) error {
	// thread.Run keeps the process main thread in a loop so AppKit can
	// deliver events. WebKitGTK runs its own loop; this is still the call
	// the driver documents for macOS, and Execute runs on main.
	return thread.Run(ctx, c.run)
}

func (c *desktopCmd) run(ctx context.Context) error {
	cfg := config.Config{
		Root:   c.Root.Value(),
		Listen: c.Listen.Value(),
		Token:  c.Token.Value(),
		Kernel: c.Kernel.Value(),
	}
	if err := auth.CheckBind(cfg.Listen, cfg.Token, false); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.RunReady(ctx, cfg, release.Version(), func(addr string) {
			ready <- addr
		})
	}()

	var addr string
	select {
	case <-ctx.Done():
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

// desktopPage is a document that navigates to the loopback server.
// The webview driver serves pages in-process on a custom scheme, which
// cannot carry the notebook WebSocket, so the window loads real HTTP.
func desktopPage(addr, token string) (string, error) {
	target, err := desktopURL(addr, token)
	if err != nil {
		return "", err
	}
	return `<!doctype html><meta charset="utf-8"><title>gaderno</title><script>location.replace(` + strconv.Quote(target) + `)</script>`, nil
}

func desktopURL(addr, token string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("listen address: %w", err)
	}
	u := url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}
	if token != "" {
		u.RawQuery = url.Values{"token": {token}}.Encode()
	}
	return u.String(), nil
}
