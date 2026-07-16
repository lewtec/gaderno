package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is set by -ldflags at release time.
var version = "dev"

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "gaderno",
	Short: "Server-authoritative collaborative notebooks",
	Long:  "gaderno runs Jupyter kernels with a server-owned CRDT notebook and thin browser clients.",
}

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	viper.SetEnvPrefix("GADERNO")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print gaderno version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}
