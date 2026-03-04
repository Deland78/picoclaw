package auth

import "github.com/spf13/cobra"

func newLoginCommand() *cobra.Command {
	var (
		provider      string
		useDeviceCode bool
		useClaudeCode bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login via OAuth or paste token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return authLoginCmd(provider, useDeviceCode, useClaudeCode)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Provider to login with (openai, anthropic)")
	cmd.Flags().BoolVar(&useDeviceCode, "device-code", false, "Use device code flow (for headless environments)")
	cmd.Flags().BoolVar(&useClaudeCode, "claude-code", false, "Use Claude Code CLI's OAuth tokens (for anthropic)")
	_ = cmd.MarkFlagRequired("provider")

	return cmd
}
