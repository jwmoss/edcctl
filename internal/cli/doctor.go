package cli

import "github.com/spf13/cobra"

func newDoctorCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify authentication and the JSON schedule endpoint",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := scheduleRange("", "", 0)
			if err != nil {
				return err
			}
			events, err := rc.client.Schedule(cmd.Context(), start, end)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"ok":         true,
				"base_url":   rc.cfg.BaseURL,
				"account_id": rc.cfg.AccountID,
				"endpoint":   "/class_calendar-ajax.php",
				"events":     len(events),
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(payload)
			}
			rc.out.Success("authenticated; JSON schedule endpoint reachable")
			rc.out.Printf("account_id: %s\nevents this week: %d\n", rc.cfg.AccountID, len(events))
			return nil
		},
	}
}
