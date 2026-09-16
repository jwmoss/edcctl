package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newScheduleCommand(rc *runtime) *cobra.Command {
	var from, to string
	var week int
	var start, end time.Time
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Read the class calendar from the JSON API",
		// Cobra validates arguments before the root hook authenticates.
		Args: func(cmd *cobra.Command, args []string) error {
			if err := usageArgs(cobra.NoArgs)(cmd, args); err != nil {
				return err
			}
			if cmd.Flags().Changed("week") && (from != "" || to != "") {
				return fmt.Errorf("%w: use --week or --from/--to, not both", errUsage)
			}
			var err error
			start, end, err = scheduleRange(from, to, week)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := rc.client.Schedule(cmd.Context(), start, end)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(events)
			}
			if rc.out.IsPlain() {
				for _, event := range events {
					rc.out.Printf("%s\n", event.Plain())
				}
				return nil
			}
			rows := make([][]string, 0, len(events))
			for _, event := range events {
				rows = append(rows, []string{event.Start, event.End, event.Title, event.Type, event.StudentID})
			}
			rc.out.Printf("%s to %s\n", start.Format("Mon Jan 2"), end.AddDate(0, 0, -1).Format("Mon Jan 2"))
			rc.out.Table([]string{"START", "END", "TITLE", "TYPE", "STUDENT ID"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "range start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "range end date, exclusive (YYYY-MM-DD)")
	cmd.Flags().IntVar(&week, "week", 0, "week offset from today; 0 is the current week")
	return cmd
}
