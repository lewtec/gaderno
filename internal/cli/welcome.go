package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/lewtec/lewkit/x/driver"
	"github.com/lewtec/lewkit/x/driver/window"
	"github.com/lewtec/lewkit/x/entry"
	"github.com/lewtec/lewkit/x/ui/gui"
	"github.com/lucasew/gaderno/internal/web"

	// The folder window and its browse button need these drivers.
	_ "github.com/lewtec/lewkit/x/driver/filedialog/prelude"
	_ "github.com/lewtec/lewkit/x/driver/window/prelude"
)

// showWelcome reports whether a real window can open.
// The memory window is always compatible and has nobody to click it.
var showWelcome = func(ctx context.Context) bool {
	handles, err := driver.List[window.Driver](ctx)
	if err != nil {
		return false
	}
	for _, handle := range handles {
		if handle.ID != "window_mem" {
			return true
		}
	}
	return false
}

// openWelcome shows the folder window. Tests replace it.
var openWelcome = func(ctx context.Context) (string, error) {
	logo, err := appLogo()
	if err != nil {
		return "", err
	}
	return gui.ChooseDir(ctx, gui.WelcomeArgs{Title: "gaderno", Logo: logo})
}

// appDir is the notebook root for an app launch.
// GADERNO_ROOT skips the folder window.
// An empty root means the user closed that window.
func appDir(ctx context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GADERNO_ROOT")); v != "" {
		return v, nil
	}
	if !showWelcome(ctx) {
		return hostRoot()
	}
	dir, err := openWelcome(ctx)
	if err != nil {
		if errors.Is(err, gui.ErrCanceled) || errors.Is(err, context.Canceled) {
			return welcomeClosed()
		}
		if errors.Is(err, driver.ErrUnavailable) {
			return hostRoot()
		}
		return "", err
	}
	if dir == "" {
		return welcomeClosed()
	}
	return dir, nil
}

func welcomeClosed() (string, error) {
	if hostNoUI() {
		entry.NotifyFail("folder window closed")
	}
	return "", nil
}

func appLogo() (image.Image, error) {
	raw, err := web.Static.ReadFile("static/logo.png")
	if err != nil {
		return nil, fmt.Errorf("logo: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("logo: %w", err)
	}
	return img, nil
}
