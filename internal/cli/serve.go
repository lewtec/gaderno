package cli

import (
	"cmp"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lucasew/gaderno/internal/app"
	"github.com/lucasew/gaderno/internal/auth"
	"github.com/lucasew/gaderno/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "Start the gaderno HTTP server",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runServe,
}

func init() {
	// Empty defaults so unset flags do not mask env (flags > env > defaults).
	serveCmd.Flags().String("root", "", "workspace root directory (env GADERNO_ROOT)")
	serveCmd.Flags().String("listen", "", "listen address (env GADERNO_LISTEN)")
	serveCmd.Flags().String("token", "", "shared access token (env GADERNO_TOKEN)")
	serveCmd.Flags().String("kernel", "", "default kernelspec name (env GADERNO_KERNEL)")
	serveCmd.Flags().Bool("i-understand", false, "allow non-loopback listen without a shared token (dangerous)")

	if err := viper.BindPFlag("root", serveCmd.Flags().Lookup("root")); err != nil {
		// best-effort
	}
	if err := viper.BindPFlag("listen", serveCmd.Flags().Lookup("listen")); err != nil {
		// best-effort
	}
	if err := viper.BindPFlag("token", serveCmd.Flags().Lookup("token")); err != nil {
		// best-effort
	}
	if err := viper.BindPFlag("kernel", serveCmd.Flags().Lookup("kernel")); err != nil {
		// best-effort
	}
	if err := viper.BindPFlag("i-understand", serveCmd.Flags().Lookup("i-understand")); err != nil {
		// best-effort
	}

	viper.SetDefault("root", ".")
	viper.SetDefault("listen", "127.0.0.1:8080")
	viper.SetDefault("token", "")
	viper.SetDefault("kernel", "python3")
	viper.SetDefault("i-understand", false)
}

// resolveServeRoot is the workspace root and kernel cwd.
// Positional `gaderno serve DIR` wins over --root / GADERNO_ROOT.
func resolveServeRoot(positional, flagOrEnv string) string {
	return cmp.Or(positional, flagOrEnv, ".")
}

func runServe(cmd *cobra.Command, args []string) error {
	positional := ""
	if len(args) == 1 {
		positional = args[0]
	}
	cfg := config.Config{
		Root:        resolveServeRoot(positional, viper.GetString("root")),
		Listen:      viper.GetString("listen"),
		Token:       viper.GetString("token"),
		Kernel:      viper.GetString("kernel"),
		IUnderstand: viper.GetBool("i-understand"),
	}

	if err := auth.CheckBind(cfg.Listen, cfg.Token, cfg.IUnderstand); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, version); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
