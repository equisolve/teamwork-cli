package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/equisolve/teamwork-cli/internal/api"
	"github.com/equisolve/teamwork-cli/internal/config"
	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/equisolve/teamwork-cli/internal/resolve"
	"github.com/spf13/cobra"
)

var (
	outputFlag string
	urlFlag    string
	tokenFlag  string
)

var rootCmd = &cobra.Command{
	Use:   "teamwork",
	Short: "CLI for Teamwork.com",
}

func Execute() {
	// Reflect the invoked name so `tw --help` says "tw", not "teamwork".
	if len(os.Args) > 0 {
		name := os.Args[0]
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if name != "" {
			rootCmd.Use = name
		}
	}
	if err := rootCmd.Execute(); err != nil {
		exitFn(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, csv")
	rootCmd.PersistentFlags().BoolP("json", "", false, "Shortcut for -o json")
	rootCmd.PersistentFlags().StringVar(&urlFlag, "url", "", "Teamwork base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "Teamwork API key (overrides config)")
}

func getClient() *api.Client {
	cfg, _ := config.Load()

	baseURL := cfg.URL
	if urlFlag != "" {
		baseURL = urlFlag
	}
	token := cfg.Token
	if tokenFlag != "" {
		token = tokenFlag
	}

	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "No Teamwork URL configured. Run: teamwork config set url https://<your>.teamwork.com")
		exitFn(1)
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "No Teamwork API key configured. Run: teamwork config set token <key>")
		exitFn(1)
	}

	return api.NewClient(baseURL, token)
}

func getOutputMode() format.OutputMode {
	if jsonFlag, _ := rootCmd.PersistentFlags().GetBool("json"); jsonFlag {
		return format.JSON
	}
	return format.ParseMode(outputFlag)
}

var resolverCache *resolve.Resolver

func getResolver() *resolve.Resolver {
	if resolverCache == nil {
		resolverCache = resolve.New(getClient())
	}
	return resolverCache
}

// decodeMap decodes JSON into map[string]interface{} preserving numbers as json.Number
// so large IDs don't render in scientific notation.
func decodeMap(data json.RawMessage) (map[string]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]interface{}
	err := dec.Decode(&m)
	return m, err
}

// exitFn is the process-exit function used by exitOnError. Tests replace it
// with one that panics so errors can be caught instead of killing the process.
var exitFn = os.Exit

func exitOnError(err error) {
	cfg, _ := config.Load()
	baseURL := cfg.URL
	if urlFlag != "" {
		baseURL = urlFlag
	}
	fmt.Fprintln(os.Stderr, "Error:", api.FormatError(err, baseURL))
	exitFn(1)
}

// formatDate normalizes a Teamwork timestamp to YYYY-MM-DD if possible.
func formatDate(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"20060102T150405Z",
		"20060102",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// joinName joins first/last into a display name.
func joinName(first, last string) string {
	return strings.TrimSpace(first + " " + last)
}
