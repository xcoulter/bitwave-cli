package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwave-io/bitwave-cli/internal/auth"
	"github.com/bitwave-io/bitwave-cli/internal/orgctx"
	"github.com/bitwave-io/bitwave-cli/internal/orgs"
)

func newConnectCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "connect ORG_ID",
		Short: "Authenticate, verify, and select a Bitwave organization",
		Long: `Connect the CLI to one Bitwave organization in a single flow.

If authentication is needed, the CLI opens Bitwave login in the local browser
and waits for the callback. It then verifies access to the exact organization,
stores it as active, and confirms the connected identity. Supplying an org ID
is an authenticated-org request; this command never completes anonymously.

Run this on the same machine as the user's browser. A remote or cloud sandbox
cannot receive the localhost OAuth callback; use a locally installed CLI or an
agent token in that environment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			orgID := strings.TrimSpace(args[0])
			if orgID == "" {
				return fmt.Errorf("organization ID is required")
			}

			if !hasStaticAuthToken() {
				credentials, err := auth.LoadCredentials()
				if err != nil {
					return fmt.Errorf("load authentication: %w", err)
				}
				if credentials == nil {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authentication required; opening Bitwave login in your browser...")
					if err := auth.Login(resolveAuthURL()); err != nil {
						return fmt.Errorf("authenticate: %w (run `bitwave connect %s` on the same machine as your browser)", err, orgID)
					}
				}
			}

			client := orgs.New(resolveCoreBaseURL(), makeOrgTokenResolver(orgID))
			org, err := client.Get(orgID)
			if err != nil {
				return fmt.Errorf("verify access to organization %s: %w", orgID, err)
			}
			if err := orgctx.Save(&orgctx.Active{OrgID: org.ID, OrgName: org.Name}); err != nil {
				return fmt.Errorf("save active organization: %w", err)
			}

			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"schemaVersion": "1",
					"status":        "connected",
					"organization":  org.ID,
					"name":          org.Name,
					"identity":      connectedIdentity(),
				})
			}
			if org.Name == "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected to Bitwave org %s.\n", org.ID)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Connected to %s (%s).\n", org.Name, org.ID)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable connection status")
	return cmd
}

func connectedIdentity() string {
	if strings.TrimSpace(os.Getenv("BITWAVE_AGENT_TOKEN")) != "" {
		return "agent"
	}
	if strings.TrimSpace(tokenFlag) != "" || strings.TrimSpace(os.Getenv("BITWAVE_TOKEN")) != "" {
		return "bearer-token"
	}
	credentials, err := auth.LoadCredentials()
	if err != nil || credentials == nil {
		return ""
	}
	return auth.ExtractEmailFromIDToken(credentials.IDToken)
}

func hasStaticAuthToken() bool {
	return strings.TrimSpace(os.Getenv("BITWAVE_AGENT_TOKEN")) != "" ||
		strings.TrimSpace(tokenFlag) != "" ||
		strings.TrimSpace(os.Getenv("BITWAVE_TOKEN")) != ""
}
