package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jwmoss/edcctl/internal/config"
	"github.com/spf13/cobra"
)

func newAppCommand(rc *runtime) *cobra.Command {
	app := &cobra.Command{
		Use: "app", Short: "Read EDC mobile-app JSON resources",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := rc.initClient(); err != nil {
				return err
			}
			if rc.cfg.AccountID != config.DefaultAccountID {
				return fmt.Errorf("app commands support only EDC account %s", config.DefaultAccountID)
			}
			return nil
		},
	}
	app.AddCommand(&cobra.Command{
		Use: "info", Short: "Read app information and studio locations", Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := rc.app.Info(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(info)
			}
			rows := [][]string{{"app_id", strconv.Itoa(info.AppID)}, {"name", info.Name}, {"ios_version", info.IOSVersion}, {"android_version", info.AndroidVersion}, {"ios_url", info.IOSURL}, {"android_url", info.AndroidURL}}
			for _, l := range info.Locations {
				rows = append(rows, []string{"location", l.Name + " (" + l.Backend + " account " + l.AccountID + ")"})
			}
			return rc.appRows([]string{"FIELD", "VALUE"}, rows)
		},
	})
	app.AddCommand(&cobra.Command{
		Use: "groups", Short: "List visible notification groups and public access status", Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			groups, err := rc.app.Groups(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(groups)
			}
			rows := make([][]string, 0, len(groups))
			for _, g := range groups {
				rows = append(rows, []string{strconv.Itoa(g.ID), g.Name, strconv.FormatBool(g.Public), strconv.Itoa(g.MemberCount)})
			}
			return rc.appRows([]string{"ID", "NAME", "PUBLIC", "MEMBERS"}, rows)
		},
	})
	var groupID int
	notifications := &cobra.Command{
		Use: "notifications --group ID", Short: "Read a public group's push notifications",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := usageArgs(cobra.NoArgs)(cmd, args); err != nil {
				return err
			}
			if groupID <= 0 {
				return fmt.Errorf("%w: --group requires a positive ID from app groups", errUsage)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			messages, err := rc.app.Notifications(cmd.Context(), groupID)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(messages)
			}
			rows := make([][]string, 0, len(messages))
			for _, m := range messages {
				rows = append(rows, []string{strconv.Itoa(m.ID), time.Unix(m.SentAt, 0).UTC().Format(time.RFC3339), m.Title, m.Message})
			}
			return rc.appRows([]string{"ID", "SENT UTC", "TITLE", "MESSAGE"}, rows)
		},
	}
	notifications.Flags().IntVar(&groupID, "group", 0, "public group ID from app groups")
	app.AddCommand(notifications)
	app.AddCommand(&cobra.Command{
		Use: "profile", Short: "Read your mobile-app profile, not the Studio Pro contact record", Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, err := rc.app.Profile(cmd.Context(), rc.cfg.Email)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(profile)
			}
			return rc.appRows([]string{"FIELD", "VALUE"}, [][]string{{"id", profile.ID}, {"first_name", profile.FirstName}, {"last_name", profile.LastName}, {"email", profile.Email}, {"date_of_birth", profile.DateOfBirth}, {"spot_tv_email", profile.SpotTVEmail}})
		},
	})
	return app
}

func (rc *runtime) appRows(headers []string, rows [][]string) error {
	for _, row := range rows {
		for i, v := range row {
			row[i] = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) {
					return ' '
				}
				return r
			}, v)
		}
		if rc.out.IsPlain() {
			rc.out.Printf("%s\n", strings.Join(row, "\t"))
		}
	}
	if !rc.out.IsPlain() {
		rc.out.Table(headers, rows)
	}
	return nil
}
