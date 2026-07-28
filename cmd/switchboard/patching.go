package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var patchingCmd = &cobra.Command{
	Use:   "patching [query]",
	Short: "Send a query to the running proxy and see the routing decision.",
	Long: `Patches a call through to the running switchboard proxy and shows what
the operator decided. Reads the query from the first argument or stdin.

Example:
  switchboard patching "Make a cinematic video of a dragon"
  echo "Make a video of a sunset" | switchboard patching`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPatching,
}

var flagPatchingTarget string

func init() {
	rootCmd.AddCommand(patchingCmd)
	patchingCmd.Flags().StringVar(&flagPatchingTarget, "target", "http://localhost:8080", "Proxy address")
}

func runPatching(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		query = strings.TrimSpace(string(data))
	}
	if query == "" {
		return fmt.Errorf("patching: no query provided (use an argument or pipe on stdin)")
	}

	body, err := json.Marshal(map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": query},
		},
	})
	if err != nil {
		return err
	}

	target := strings.TrimRight(flagPatchingTarget, "/") + "/v1/chat/completions"
	resp, err := http.Post(target, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("patching: dial %s: %w", flagPatchingTarget, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	reason := resp.Header.Get("X-Switchboard-Reason")
	provider := resp.Header.Get("X-Operator-Provider")
	model := resp.Header.Get("X-Operator-Model")
	tags := resp.Header.Get("X-Operator-Tags")

	fmt.Fprintf(cmd.OutOrStdout(), "query    %q\n", truncateQuery(query, 72))
	fmt.Fprintf(cmd.OutOrStdout(), "status   %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	if provider != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "provider %s\n", provider)
	}
	if model != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "model    %s\n", model)
	}
	if tags != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "tags     %s\n", tags)
	}
	if reason != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "reason   %s\n", reason)
	}
	if len(respBody) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "response %s\n", string(respBody))
	}
	return nil
}

func truncateQuery(q string, n int) string {
	lines := strings.SplitN(q, "\n", 2)
	q = lines[0]
	if len(q) <= n {
		return q
	}
	return q[:n-1] + "…"
}
