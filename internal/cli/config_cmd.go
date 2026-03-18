package cli

import (
	"encoding/json"
	"fmt"

	"github.com/muhammed/mutercim/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show effective configuration (merged config + flags + defaults)",
		RunE:  runConfig,
	}
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
