package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show effective configuration (merged config + flags + defaults)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var data []byte
			switch format {
			case "json":
				data, err = json.MarshalIndent(cfg, "", "  ")
			case "yaml":
				var buf bytes.Buffer
				enc := yaml.NewEncoder(&buf)
				enc.SetIndent(2)
				err = enc.Encode(cfg)
				data = buf.Bytes()
			default:
				return fmt.Errorf("unsupported format %q (use json or yaml)", format)
			}
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			fmt.Print(string(data))
			if format == "json" {
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "output format: yaml, json")
	return cmd
}
