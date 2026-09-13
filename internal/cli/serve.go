package cli

import (
	"cmp"
	"context"
	"fmt"

	"github.com/lewtec/lewkit/x/cmd"
	"github.com/lewtec/lewkit/x/release"
	"github.com/lucasew/gaderno/internal/app"
	"github.com/lucasew/gaderno/internal/auth"
	"github.com/lucasew/gaderno/internal/config"
)

type serveCmd struct {
	Root        cmd.WorkDirArg  `long:"root" env:"GADERNO_ROOT" help:"workspace root directory"`
	Listen      cmd.AddrArg     `long:"listen" env:"GADERNO_LISTEN,PORT" default:"127.0.0.1:8080" help:"listen address"`
	Token       cmd.StringArg   `long:"token" env:"GADERNO_TOKEN" default:"" help:"shared access token"`
	Kernel      cmd.StringArg   `long:"kernel" env:"GADERNO_KERNEL" default:"python3" help:"default kernelspec name"`
	IUnderstand cmd.Flag        `long:"i-understand" env:"GADERNO_I_UNDERSTAND" help:"allow non-loopback listen without a shared token (dangerous)"`
	Dir         *cmd.DataDirArg `help:"workspace root (wins over --root)"`
}

func (serveCmd) Description() string {
	return "start the gaderno HTTP server"
}

// resolveServeRoot is the workspace root and kernel cwd.
// Positional `gaderno serve DIR` wins over --root / GADERNO_ROOT.
func resolveServeRoot(positional, flagOrEnv string) string {
	return cmp.Or(positional, flagOrEnv, ".")
}

func (c *serveCmd) Run(ctx context.Context) error {
	positional := ""
	if c.Dir != nil {
		positional = c.Dir.Value()
	}
	cfg := config.Config{
		Root:        resolveServeRoot(positional, c.Root.Value()),
		Listen:      c.Listen.Value(),
		Token:       c.Token.Value(),
		Kernel:      c.Kernel.Value(),
		IUnderstand: c.IUnderstand.Value(),
	}

	if err := auth.CheckBind(cfg.Listen, cfg.Token, cfg.IUnderstand); err != nil {
		return err
	}

	if err := app.Run(ctx, cfg, release.Version()); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
