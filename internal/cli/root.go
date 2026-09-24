package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/lewtec/lewkit/x/cmd"
)

type root struct {
	Serve   *serveCmd
	Desktop *desktopCmd
	Version *cmd.VersionCmd
}

func (root) Description() string {
	return "gaderno runs Jupyter kernels with a server-owned CRDT notebook and thin browser clients."
}

func (root) Run(context.Context) error {
	text, err := cmd.Usage[cmd.App[root]]("gaderno")
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(text)
	return err
}

// Execute parses os.Args and runs the selected command.
func Execute(ctx context.Context) error {
	app, err := cmd.Parse[cmd.App[root]](os.Args[1:]...)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return app.Run(ctx)
}
