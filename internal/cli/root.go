package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string
	logFile  string
	pages    string
)

// NewRootCmd creates the root cobra command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mutercim",
		Short: "Translate Arabic Islamic scholarly books into Turkish",
		Long: `mutercim (مترجم) — a CLI tool that translates Arabic Islamic scholarly books
into Turkish, preserving layout, structure, and domain-specific terminology.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load .env and .envrc from current directory
			loadEnvFile(".env")
			loadEnvFile(".envrc")
			return nil
		},
	}

	// Common flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file (default: ./mutercim.yaml)")
	rootCmd.PersistentFlags().StringVarP(&pages, "pages", "p", "", "page range: \"1-50\", \"1,5,10-20\", \"all\" (default: from config or all)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log verbosity: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "also write logs to this file")

	// Command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "pipeline", Title: "Pipeline Commands:"},
		&cobra.Group{ID: "workspace", Title: "Workspace Commands:"},
	)

	// Pipeline commands — ordered by phase
	addToGroup(rootCmd, "pipeline",
		ordered(1, newRunCmd()),
		ordered(2, newExtractCmd()),
		ordered(3, newEnrichCmd()),
		ordered(4, newTranslateCmd()),
		ordered(5, newCompileCmd()),
		ordered(6, newValidateCmd()),
	)

	// Workspace commands
	addToGroup(rootCmd, "workspace",
		ordered(1, newInitCmd()),
		ordered(2, newStatusCmd()),
		ordered(3, newConfigCmd()),
		ordered(4, newKnowledgeCmd()),
	)

	// Custom help template that respects our ordering
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

// loadEnvFile reads a .env or .envrc file and sets environment variables.
// Supports KEY=VALUE and export KEY=VALUE lines. Skips comments and blank lines.
// Does not override variables already set in the environment.
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // file not found is fine
	}
	for _, line := range strings.Split(string(data), "\n") {
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
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
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

func init() {
	cobra.AddTemplateFunc("formatGroupedCommands", formatGroupedCommands)
}
