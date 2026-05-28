package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/equisolve/teamwork-cli/internal/format"
	"github.com/spf13/cobra"
)

var (
	apiMethod string
	apiQuery  []string
	apiData   string
)

var apiCmd = &cobra.Command{
	Use:   "api <path>",
	Short: "Make a raw authenticated request to the Teamwork API",
	Long: `Make a raw authenticated request to any Teamwork API endpoint.

The path is everything after your Teamwork base URL, e.g. "/projects.json"
or "projects/api/v3/projects.json". Authentication, base URL, and JSON
decoding are handled for you; the raw response body is printed.

Examples:
  teamwork api /projects.json
  teamwork api /me.json
  teamwork api -q "status=active" -q "page=2" /projects.json
  teamwork api -X POST -d '{"todo-item":{"content":"Hi"}}' /tasks/123/todo-items.json
  teamwork api -X PUT -d @payload.json /tasks/123.json
  echo '{...}' | teamwork api -X POST -d - /tasks/123/todo-items.json
  teamwork api -X DELETE /tasks/123.json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		mode := getOutputMode()

		params := url.Values{}
		for _, q := range apiQuery {
			k, v, found := strings.Cut(q, "=")
			if !found {
				fmt.Fprintf(os.Stderr, "Invalid query parameter %q (expected key=value)\n", q)
				exitFn(1)
			}
			params.Add(k, v)
		}

		body, err := resolveBody(apiData)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			exitFn(1)
		}

		data, err := client.Raw(apiMethod, args[0], params, body)
		if err != nil {
			exitOnError(err)
		}

		if mode == format.JSON || mode == format.Table {
			format.PrintJSON(data)
			return
		}
		fmt.Println(string(data))
	},
}

// resolveBody interprets the -d value: "@file" reads from a file, "-" reads
// from stdin, "" means no body, and anything else is used literally.
func resolveBody(d string) (string, error) {
	switch {
	case d == "":
		return "", nil
	case d == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("could not read body from stdin: %w", err)
		}
		return string(b), nil
	case strings.HasPrefix(d, "@"):
		b, err := os.ReadFile(d[1:])
		if err != nil {
			return "", fmt.Errorf("could not read body file: %w", err)
		}
		return string(b), nil
	default:
		return d, nil
	}
}

func init() {
	apiCmd.Flags().StringVarP(&apiMethod, "method", "X", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	apiCmd.Flags().StringArrayVarP(&apiQuery, "query", "q", nil, "Query parameter key=value (repeatable)")
	apiCmd.Flags().StringVarP(&apiData, "data", "d", "", "Request body: literal JSON, @file, or - for stdin")
	rootCmd.AddCommand(apiCmd)
}
