package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate or install shell completions",
		Long: `Generate or install shell completions for bash, zsh, or fish.

  mutercim completion install    Auto-detect shell and install
  mutercim completion bash       Print bash completion script
  mutercim completion zsh        Print zsh completion script
  mutercim completion fish       Print fish completion script`,
	}

	cmd.AddCommand(newCompletionInstallCmd(rootCmd))
	cmd.AddCommand(newCompletionGenCmd(rootCmd, "bash"))
	cmd.AddCommand(newCompletionGenCmd(rootCmd, "zsh"))
	cmd.AddCommand(newCompletionGenCmd(rootCmd, "fish"))

	return cmd
}

func newCompletionGenCmd(rootCmd *cobra.Command, shell string) *cobra.Command {
	return &cobra.Command{
		Use:   shell,
		Short: fmt.Sprintf("Print %s completion script to stdout", shell),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch shell {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s", shell)
			}
		},
	}
}

func newCompletionInstallCmd(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Auto-detect shell and install completions",
		Long: `Detects your shell from $SHELL and installs the completion script.

Locations:
  zsh:  ~/.zfunc/_mutercim
  bash: ~/.local/share/bash-completion/completions/mutercim
  fish: ~/.config/fish/completions/mutercim.fish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := detectShell()
			if shell == "" {
				return fmt.Errorf("could not detect shell from $SHELL — use 'mutercim completion bash/zsh/fish' and redirect to a file")
			}
			return installCompletion(rootCmd, shell)
		},
	}
}

// detectShell returns "bash", "zsh", or "fish" from the SHELL env var.
func detectShell() string {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "bash", "zsh", "fish":
		return shell
	default:
		return ""
	}
}

func installCompletion(rootCmd *cobra.Command, shell string) error {
	var buf bytes.Buffer
	var err error

	switch shell {
	case "zsh":
		err = rootCmd.GenZshCompletion(&buf)
	case "bash":
		err = rootCmd.GenBashCompletion(&buf)
	case "fish":
		err = rootCmd.GenFishCompletion(&buf, true)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	if err != nil {
		return fmt.Errorf("generate %s completion: %w", shell, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	var destPath string
	var postInstall string

	switch shell {
	case "zsh":
		dir := filepath.Join(home, ".zfunc")
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		destPath = filepath.Join(dir, "_mutercim")
		postInstall = "Add to ~/.zshrc if not already present:\n  fpath=(~/.zfunc $fpath)\n  autoload -Uz compinit && compinit\nThen restart your shell or run: source ~/.zshrc"

	case "bash":
		dir := filepath.Join(home, ".local", "share", "bash-completion", "completions")
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		destPath = filepath.Join(dir, "mutercim")
		postInstall = "Restart your shell or run: source " + destPath

	case "fish":
		dir := filepath.Join(home, ".config", "fish", "completions")
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		destPath = filepath.Join(dir, "mutercim.fish")
		postInstall = "Completions will be loaded automatically on next shell start."
	}

	if err := os.WriteFile(destPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("write %s: %w", destPath, err)
	}

	fmt.Printf("Installed %s completions to %s\n", shell, destPath)
	for _, line := range strings.Split(postInstall, "\n") {
		fmt.Printf("  %s\n", line)
	}
	return nil
}
