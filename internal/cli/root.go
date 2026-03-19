package cli

import (
	"fmt"
	"os"

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
	}

	// Common flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file (default: ./mutercim.yaml)")
	rootCmd.PersistentFlags().StringVarP(&pages, "pages", "p", "", "page range: \"1-50\", \"1,5,10-20\", \"all\" (default: from config or all)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log verbosity: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "also write logs to this file")

	// Register subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newExtractCmd())
	rootCmd.AddCommand(newEnrichCmd())
	rootCmd.AddCommand(newTranslateCmd())
	rootCmd.AddCommand(newCompileCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newKnowledgeCmd())

	return rootCmd
}

// Execute runs the root command.
func Execute() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
