package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	logLevel  string
	pages     string
	outputDir string
	auto      bool
	force     bool
)

// NewRootCmd creates the root cobra command.
//nolint:cyclop,gocognit,funlen // root command wiring with many flags and subcommands
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mutercim",
		Short: "Translate Islamic scholarly books between languages",
		Long: `mutercim (مترجم) — a CLI tool that translates Islamic scholarly books
between languages, preserving layout, structure, and domain-specific terminology.`,
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load .env and .envrc from current directory
			loadEnvFile(".env")
			loadEnvFile(".envrc")

			// Set up log file (non-fatal if workspace not found)
			var fileLogger *slog.Logger
			var logFileHandle *os.File
			ws, wsErr := workspace.Discover(".")
			if wsErr == nil {
				// Resolve output dir from config so log goes to the right place
				configPath := cfgFile
				if configPath == "" {
					configPath = ws.ConfigPath()
				}
				if cfg, err := config.Load(configPath); err == nil {
					applyOutputDir(ws, cfg)
					if logLevel == "" {
						logLevel = cfg.LogLevel
					}
				}
				if ws.OutputDir != ws.Root {
					_ = os.MkdirAll(ws.OutputDir, 0750)
				}

				// Resolve log level: CLI flag > config > default "info"
				if logLevel == "" {
					logLevel = "info"
				}
				level := parseLogLevel(logLevel)

				f, err := os.OpenFile(ws.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
				if err == nil {
					logFileHandle = f
					fileLogger = slog.New(newHumanHandler(f, level))
				} else {
					fmt.Fprintf(os.Stderr, "warning: cannot open log file %s: %v\n", ws.LogPath(), err)
				}
			}
			if logLevel == "" {
				logLevel = "info"
			}
			if fileLogger == nil {
				fileLogger = slog.New(newHumanHandler(io.Discard, parseLogLevel(logLevel)))
			}
			slog.SetDefault(fileLogger)

			slog.Info("─── mutercim started", "command", cmd.Name(), "args", strings.Join(os.Args[1:], " ")) //nolint:gosec // G706: logging CLI args is intentional; log file is user-private

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
					_ = logFileHandle.Close()
				}
				return nil
			}

			return nil
		},
	}

	// Common flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file (default: ./mutercim.yaml)")
	rootCmd.PersistentFlags().StringVarP(&pages, "pages", "p", "", "page range: \"1-50\", \"1,5,10-20\", \"all\" (default: from config or all)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "", "log verbosity: debug, info, warn, error (default: from config or info)")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "", "output directory (default: .)")
	rootCmd.PersistentFlags().BoolVarP(&auto, "auto", "a", false, "auto-run missing prerequisite phases before the requested phase")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "force re-processing of already completed pages")

	// Command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "pipeline", Title: "Pipeline Commands:"},
		&cobra.Group{ID: "workspace", Title: "Workspace Commands:"},
	)

	// Pipeline commands — ordered by phase
	addToGroup(rootCmd, "pipeline",
		ordered(1, newAllCmd()),
		ordered(2, newCutCmd()),
		ordered(3, newLayoutCmd()),
		ordered(4, newOCRCmd()),
		ordered(5, newReadCmd()),
		ordered(6, newSolveCmd()),
		ordered(7, newTranslateCmd()),
		ordered(8, newWriteCmd()),
	)

	// Workspace commands
	addToGroup(rootCmd, "workspace",
		ordered(1, newInitCmd()),
		ordered(2, newStatusCmd()),
		ordered(3, newConfigCmd()),
		ordered(4, newCleanCmd()),
		ordered(5, newCompletionCmd(rootCmd)),
	)

	// Custom help template with colors
	colors := display.NewStatusColors(os.Stdout)
	cobra.AddTemplateFunc("formatGroupedCommands", makeFormatGroupedCommands(colors))
	cobra.AddTemplateFunc("cyan", colors.Cyan)
	cobra.AddTemplateFunc("bold", colors.Bold)
	cobra.AddTemplateFunc("dim", colors.Dim)
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

// resolveInputPaths returns absolute paths for all configured inputs.
func resolveInputPaths(ws *workspace.Workspace, cfg *config.Config) []string {
	paths := make([]string, len(cfg.Inputs))
	for i, inp := range cfg.Inputs {
		paths[i] = cfg.ResolvePath(ws.Root, inp.Path)
	}
	return paths
}

// applyOutputDir sets ws.OutputDir from the config's output field and CLI --output flag.
// CLI flag takes precedence over config.
func applyOutputDir(ws *workspace.Workspace, cfg *config.Config) {
	output := cfg.Output
	if outputDir != "" {
		output = outputDir
		cfg.Output = output
	}
	if output != "" && output != "." {
		ws.OutputDir = cfg.ResolvePath(ws.Root, output)
	}
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		colors := display.NewStatusColors(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %v\n", colors.Red("Error:"), err)
		os.Exit(1)
	}
}

// makeFormatGroupedCommands returns a template function that formats command groups with colors.
func makeFormatGroupedCommands(colors display.StatusColors) func(*cobra.Command) string {
	return func(cmd *cobra.Command) string {
		var b strings.Builder

		for _, group := range cmd.Groups() {
			cmds := commandsInGroup(cmd.Commands(), group.ID)
			if len(cmds) == 0 {
				continue
			}
			fmt.Fprintf(&b, "%s\n", colors.Bold(group.Title))
			for _, c := range cmds {
				fmt.Fprintf(&b, "  %s %s\n", colors.Cyan(fmt.Sprintf("%-12s", c.Name())), c.Short)
			}
			b.WriteString("\n")
		}

		ungrouped := ungroupedCommands(cmd.Commands())
		if len(ungrouped) > 0 {
			fmt.Fprintf(&b, "%s\n", colors.Bold("Additional Commands:"))
			for _, c := range ungrouped {
				fmt.Fprintf(&b, "  %s %s\n", colors.Cyan(fmt.Sprintf("%-12s", c.Name())), c.Short)
			}
			b.WriteString("\n")
		}

		return b.String()
	}
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
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is a well-known config file, not user HTTP input
	if err != nil {
		return // file not found is fine
	}
	for key, value := range parseEnvLines(string(data)) {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

// parseEnvLines parses KEY=VALUE and export KEY=VALUE lines from env file content.
// Skips comments and blank lines. Strips surrounding quotes from values.
//nolint:cyclop,gocognit // env file parsing with quote/comment handling
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
		// Strip inline comments: only if # is preceded by whitespace
		// (but not inside quotes)
		if value == "" || (value[0] != '"' && value[0] != '\'') {
			if idx := strings.Index(value, " #"); idx >= 0 {
				value = strings.TrimSpace(value[:idx])
			}
		}
		// Remove surrounding quotes
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
		result[key] = value
	}
	return result
}

// humanHandler is a slog.Handler that writes human-readable log lines.
// Format: "15:04:05 LEVEL msg  key=value key=value"
type humanHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
}

func newHumanHandler(w io.Writer, level slog.Level) *humanHandler {
	return &humanHandler{w: w, level: level}
}

func (h *humanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *humanHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format("15:04:05"))
	b.WriteByte(' ')
	switch r.Level {
	case slog.LevelDebug:
		b.WriteString("DEBUG")
	case slog.LevelInfo:
		b.WriteString("INFO ")
	case slog.LevelWarn:
		b.WriteString("WARN ")
	case slog.LevelError:
		b.WriteString("ERROR")
	default:
		b.WriteString(r.Level.String())
	}
	b.WriteString("  ")
	b.WriteString(r.Message)
	// Write pre-set attrs
	for _, a := range h.attrs {
		fmt.Fprintf(&b, "  %s=%v", a.Key, a.Value)
	}
	// Write per-record attrs
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, "  %s=%v", a.Key, a.Value)
		return true
	})
	b.WriteByte('\n')
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &humanHandler{w: h.w, level: h.level, attrs: append(h.attrs, attrs...)}
}

func (h *humanHandler) WithGroup(name string) slog.Handler {
	// Groups not used in this codebase; passthrough
	return h
}

var usageTemplate = `{{ bold "Usage:" }}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{ bold "Aliases:" }}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{ bold "Examples:" }}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{ formatGroupedCommands . }}{{end}}{{if .HasAvailableLocalFlags}}{{ bold "Flags:" }}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{ bold "Global Flags:" }}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{ bold "Additional help topics:" }}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

{{ dim "Use \"" }}{{ dim .CommandPath }}{{ dim " [command] --help\" for more information about a command." }}{{end}}
`
