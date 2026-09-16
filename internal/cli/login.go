package cli

import (
	"github.com/spf13/cobra"
)

func newLoginCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Validate the configured Studio Pro credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{
				"ok":         true,
				"base_url":   rc.cfg.BaseURL,
				"account_id": rc.cfg.AccountID,
				"email":      rc.cfg.Email,
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(payload)
			}
			rc.out.Success("login successful")
			rc.out.Printf("base_url: %s\n", rc.cfg.BaseURL)
			rc.out.Printf("account_id: %s\n", rc.cfg.AccountID)
			rc.out.Printf("email: %s\n", rc.cfg.Email)
			return nil
		},
	}
}
