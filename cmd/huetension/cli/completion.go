package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompletionCmd wires shell-completion script generation. cobra produces
// the actual scripts; this command is a thin dispatcher with one
// subcommand per supported shell. The user pipes the output into a file or
// sources it directly:
//
//	huetension completion bash > /etc/bash_completion.d/huetension
//	huetension completion zsh  > "${fpath[1]}/_huetension"
//	huetension completion fish | source
//	huetension completion powershell | Out-String | Invoke-Expression
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion {bash|zsh|fish|powershell}",
		Short: "Generate shell completion scripts",
		Long: "Output a shell-completion script for the chosen shell. Pipe the result into your shell's " +
			"completion directory (or source it on the fly).\n\nBash:\n  $ source <(huetension completion bash)\n\n" +
			"Zsh:\n  $ huetension completion zsh > \"${fpath[1]}/_huetension\"\n\nFish:\n  $ huetension completion fish | source\n\n" +
			"PowerShell:\n  PS> huetension completion powershell | Out-String | Invoke-Expression",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(stdoutWriter, true)
			case "zsh":
				return root.GenZshCompletion(stdoutWriter)
			case "fish":
				return root.GenFishCompletion(stdoutWriter, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(stdoutWriter)
			}
			return fmt.Errorf("unsupported shell %q", args[0])
		},
	}
	return cmd
}
