package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string
	pages    string
)

// NewRootCmd creates the root cobra command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mutercim",
		Short: "Translate Islamic scholarly books between languages",
		Long: `mutercim (مترجم) — a CLI tool that translates Islamic scholarly books
between languages, preserving layout, structure, and domain-specific terminology.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load .env and .envrc from current directory
			loadEnvFile(".env")
			loadEnvFile(".envrc")

			// Parse log level
			level := parseLogLevel(logLevel)

			// Set up log file (non-fatal if workspace not found)
			var fileLogger *slog.Logger
			var logFileHandle *os.File
			ws, wsErr := workspace.Discover(".")
			if wsErr == nil {
				logPath := filepath.Join(ws.Root, "mutercim.log")
				f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err == nil {
					logFileHandle = f
					fileLogger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}))
				}
			}
			if fileLogger == nil {
				fileLogger = slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: level}))
			}
			slog.SetDefault(fileLogger)

			// Create display (writes progress to stderr)
			disp := display.New(os.Stderr, nil)
			ctx := display.WithDisplay(cmd.Context(), disp)

			// Set up Ctrl+C handling
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
			cmd.SetContext(ctx)

			// Register cleanup for log file and signal handling
			cmd.Root().PersistentPostRunE = func(*cobra.Command, []string) error {
				stop()
				if logFileHandle != nil {
					logFileHandle.Close()
				}
				return nil
			}

			return nil
		},
	}

	// Common flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file (default: ./mutercim.yaml)")
	rootCmd.PersistentFlags().StringVarP(&pages, "pages", "p", "", "page range: \"1-50\", \"1,5,10-20\", \"all\" (default: from config or all)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log verbosity: debug, info, warn, error")

	// Command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "pipeline", Title: "Pipeline Commands:"},
		&cobra.Group{ID: "workspace", Title: "Workspace Commands:"},
	)

	// Pipeline commands — ordered by phase
	addToGroup(rootCmd, "pipeline",
		ordered(1, newMakeCmd()),
		ordered(2, newPagesCmd()),
		ordered(3, newReadCmd()),
		ordered(4, newSolveCmd()),
		ordered(5, newTranslateCmd()),
		ordered(6, newWriteCmd()),
		ordered(7, newValidateCmd()),
	)

	// Workspace commands
	addToGroup(rootCmd, "workspace",
		ordered(1, newInitCmd()),
		ordered(2, newStatusCmd()),
		ordered(3, newConfigCmd()),
		ordered(4, newKnowledgeCmd()),
	)

	// Custom help template that respects our ordering
	cobra.AddTemplateFunc("formatGroupedCommands", formatGroupedCommands)
	rootCmd.SetUsageTemplate(usageTemplate)

	return rootCmd
}

func addToGroup(root *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = groupID
		root.AddCommand(cmd)
	}
}

// ordered sets a sort annotation so commands display in the given order.
func ordered(n int, cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations["order"] = fmt.Sprintf("%02d", n)
	return cmd
}

// commandsByOrder sorts commands by their "order" annotation, falling back to name.
func commandsByOrder(cmds []*cobra.Command) []*cobra.Command {
	sorted := make([]*cobra.Command, len(cmds))
	copy(sorted, cmds)
	sort.SliceStable(sorted, func(i, j int) bool {
		oi := sorted[i].Annotations["order"]
		oj := sorted[j].Annotations["order"]
		if oi != oj {
			return oi < oj
		}
		return sorted[i].Name() < sorted[j].Name()
	})
	return sorted
}

// commandsInGroup returns commands matching groupID, sorted by order annotation.
func commandsInGroup(cmds []*cobra.Command, groupID string) []*cobra.Command {
	var filtered []*cobra.Command
	for _, cmd := range cmds {
		if cmd.GroupID == groupID && cmd.IsAvailableCommand() {
			filtered = append(filtered, cmd)
		}
	}
	return commandsByOrder(filtered)
}

// ungroupedCommands returns available commands with no group, sorted by order.
func ungroupedCommands(cmds []*cobra.Command) []*cobra.Command {
	var filtered []*cobra.Command
	for _, cmd := range cmds {
		if cmd.GroupID == "" && cmd.IsAvailableCommand() && cmd.Name() != "help" {
			filtered = append(filtered, cmd)
		}
	}
	return commandsByOrder(filtered)
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// formatGroupedCommands builds the help text for all command groups.
func formatGroupedCommands(cmd *cobra.Command) string {
	var b strings.Builder

	for _, group := range cmd.Groups() {
		cmds := commandsInGroup(cmd.Commands(), group.ID)
		if len(cmds) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n", group.Title)
		for _, c := range cmds {
			fmt.Fprintf(&b, "  %-12s %s\n", c.Name(), c.Short)
		}
		b.WriteString("\n")
	}

	ungrouped := ungroupedCommands(cmd.Commands())
	if len(ungrouped) > 0 {
		b.WriteString("Additional Commands:\n")
		for _, c := range ungrouped {
			fmt.Fprintf(&b, "  %-12s %s\n", c.Name(), c.Short)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// loadEnvFile reads a .env or .envrc file and sets environment variables.
// Does not override variables already set in the environment.
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // file not found is fine
	}
	for key, value := range parseEnvLines(string(data)) {
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// parseEnvLines parses KEY=VALUE and export KEY=VALUE lines from env file content.
// Skips comments and blank lines. Strips surrounding quotes from values.
func parseEnvLines(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Remove surrounding quotes
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		result[key] = value
	}
	return result
}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{ formatGroupedCommands . }}{{end}}{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
