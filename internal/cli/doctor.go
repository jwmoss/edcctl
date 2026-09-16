package cli

import (
	"regexp"

	"github.com/spf13/cobra"
)

var titlePattern = regexp.MustCompile(`(?is)<title>([^<]*)</title>`)

func newDoctorCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify configuration and portal connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The client logs in during startup; fetching a portal page
			// confirms the session is usable.
			page, err := rc.client.Get(cmd.Context(), "/app_more.php", nil)
			if err != nil {
				return err
			}
			studio := ""
			if match := titlePattern.FindSubmatch(page); match != nil {
				studio = string(match[1])
			}
			payload := map[string]any{
				"ok":         true,
				"base_url":   rc.cfg.BaseURL,
				"account_id": rc.cfg.AccountID,
				"studio":     studio,
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(payload)
			}
			rc.out.Success("portal reachable")
			rc.out.Printf("base_url: %s\n", rc.cfg.BaseURL)
			rc.out.Printf("account_id: %s\n", rc.cfg.AccountID)
			if studio != "" {
				rc.out.Printf("studio: %s\n", studio)
			}
			return nil
		},
	}
}
