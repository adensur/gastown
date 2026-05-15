package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/tmux"
)

func init() {
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugIdleCheckCmd)
}

var debugCmd = &cobra.Command{
	Use:     "debug",
	GroupID: GroupDiag,
	Short:   "Diagnostic helpers",
	Long:    "Diagnostic helpers for inspecting gt internals. Subcommands are stable for human use but their output is not API.",
}

var debugIdleCheckCmd = &cobra.Command{
	Use:   "idle-check [session]",
	Short: "Print idle-detection verdict for a tmux session",
	Long: `Captures the bottom of a tmux pane and prints whether gt currently considers
the session idle for nudge delivery. With no argument, uses the current
session. Useful for validating idle/input-empty detection against live
Claude Code panes.

Output fields:
  prompt_visible   prompt prefix found in capture
  busy             "esc to interrupt" present in capture
  input_has_content input box has user-typed text (gt-o3w gate)
  verdict_loose    WaitForIdle would succeed (existing behavior)
  verdict_strict   WaitForIdleWithOpts{RequireEmptyInput:true} would succeed`,
	RunE: runDebugIdleCheck,
}

func runDebugIdleCheck(cmd *cobra.Command, args []string) error {
	t := tmux.NewTmux()
	var session string
	if len(args) > 0 {
		session = args[0]
	} else {
		s, err := t.ResolveCurrentSession()
		if err != nil {
			return fmt.Errorf("resolving current session: %w (pass a session name as the first argument)", err)
		}
		session = s
	}

	lines, err := t.CapturePaneLines(session, 10)
	if err != nil {
		return fmt.Errorf("capturing pane for %s: %w", session, err)
	}

	promptPrefix := tmux.DefaultReadyPromptPrefix
	prefix := strings.TrimSpace(promptPrefix)

	busy := false
	for _, line := range lines {
		if strings.Contains(strings.TrimSpace(line), "esc to interrupt") {
			busy = true
			break
		}
	}

	promptVisible := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Mirror matchesPromptPrefix logic without exporting it.
		normalized := strings.ReplaceAll(trimmed, " ", " ")
		normalizedPrefix := strings.ReplaceAll(promptPrefix, " ", " ")
		if strings.HasPrefix(normalized, normalizedPrefix) || (prefix != "" && normalized == prefix) {
			promptVisible = true
			break
		}
	}

	inputHasContent := tmux.HasInputContent(lines, promptPrefix)
	verdictLoose := promptVisible && !busy
	verdictStrict := verdictLoose && !inputHasContent

	fmt.Fprintf(os.Stdout, "session: %s\n", session)
	fmt.Fprintf(os.Stdout, "prompt_visible:    %v\n", promptVisible)
	fmt.Fprintf(os.Stdout, "busy:              %v\n", busy)
	fmt.Fprintf(os.Stdout, "input_has_content: %v\n", inputHasContent)
	fmt.Fprintf(os.Stdout, "verdict_loose:     %v  (current WaitForIdle)\n", verdictLoose)
	fmt.Fprintf(os.Stdout, "verdict_strict:    %v  (with RequireEmptyInput)\n", verdictStrict)
	return nil
}
