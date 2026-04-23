package cmd

import (
	"fmt"
	"os"

	"github.com/equisolve/teamwork-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value (url, token)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.Set(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			exitFn(1)
		}
		fmt.Printf("Set %s in %s\n", args[0], config.Path())
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		val, err := config.Get(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			exitFn(1)
		}
		if val == "" {
			fmt.Println("(not set)")
		} else {
			fmt.Println(val)
		}
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all config values",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _ := config.Load()
		fmt.Printf("Config file: %s\n", config.Path())
		fmt.Printf("url:   %s\n", displayValue(cfg.URL))
		fmt.Printf("token: %s\n", maskToken(cfg.Token))
	},
}

func displayValue(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}

func maskToken(t string) string {
	if t == "" {
		return "(not set)"
	}
	if len(t) <= 8 {
		return "****"
	}
	return t[:4] + "..." + t[len(t)-4:]
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configShowCmd)
	rootCmd.AddCommand(configCmd)
}
