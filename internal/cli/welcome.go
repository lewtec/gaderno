package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"strings"

	"github.com/lewtec/lewkit/x/app"
	"github.com/lewtec/lewkit/x/driver"
	"github.com/lewtec/lewkit/x/driver/window"
	"github.com/lewtec/lewkit/x/entry"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lewtec/lewkit/x/ui/gui"
	"github.com/lucasew/gaderno/internal/web"

	// Browse on the folder window calls filedialog.Choose.
	_ "github.com/lewtec/lewkit/x/driver/filedialog/prelude"
)

const (
	folderTitle  = "gaderno"
	folderWidth  = 880
	folderHeight = 720
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
// An empty path and a nil error means the user closed it.
var openWelcome = func(ctx context.Context) (string, error) {
	dirs, err := gui.Recent()
	if err != nil {
		dirs = nil
	}
	logo, err := appLogo()
	if err != nil {
		return "", err
	}
	model := gui.NewWelcome(gui.WelcomeArgs{Title: folderTitle, Dirs: dirs, Logo: logo})
	err = runFolderWindow(ctx, model)
	if path := model.Picked(); path != "" {
		return path, nil
	}
	if ctx.Err() != nil {
		return "", context.Cause(ctx)
	}
	if err != nil {
		return "", err
	}
	return "", nil
}

// runFolderWindow opens the welcome model and blocks until it closes.
// A stamped release uses app.App so Android keeps the UI loop and drops
// the splash. app.App panics without a stamp, and a desktop host with
// ELETROCROMO_NO_UI rejects a GUI model. Those launches use app.Open.
func runFolderWindow(ctx context.Context, model *gui.Welcome) error {
	if releaseStamped() && folderApp() {
		return app.App{
			Title:   folderTitle,
			Width:   folderWidth,
			Height:  folderHeight,
			Handler: app.GUI(model),
		}.Run(ctx)
	}
	if runtime.GOOS == "android" {
		entry.ShowSurface()
	}
	return app.Open(ctx, app.GUI(model), folderTitle, folderWidth, folderHeight)
}

// folderApp reports whether app.App can show the folder window.
// Android leaves a GUI model on the surface. Other no-UI hosts require a web handler.
func folderApp() bool {
	if !hostNoUI() {
		return true
	}
	return runtime.GOOS == "android"
}

func releaseStamped() bool {
	version := strings.TrimSpace(release.Version())
	if version == "" || version == "dev" || strings.HasPrefix(version, "dev-") {
		return false
	}
	_, err := release.AppID()
	return err == nil
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
		if errors.Is(err, context.Canceled) {
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
	if !strings.HasPrefix(dir, "content:") {
		if err := gui.Remember(dir); err != nil {
			return "", err
		}
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
