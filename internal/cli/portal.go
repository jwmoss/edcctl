package cli

import (
	"github.com/spf13/cobra"
)

func newStudentsCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "students",
		Short:   "List students and their enrolled classes",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			students, err := rc.client.Students(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(students)
			}
			for _, student := range students {
				rc.out.Printf("%s (age %s)\n", student.Name, student.Age)
				rows := make([][]string, 0, len(student.Classes))
				for _, class := range student.Classes {
					rows = append(rows, []string{
						class.Day, class.Time, class.Name, class.Instructor, class.Room, class.Dates,
					})
				}
				rc.out.Table([]string{"DAY", "TIME", "CLASS", "INSTRUCTOR", "ROOM", "DATES"}, rows)
				rc.out.Println("")
			}
			return nil
		},
	}
}

func newScheduleCommand(rc *runtime) *cobra.Command {
	var (
		from string
		to   string
		week int
	)
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Show the class calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := scheduleRange(from, to, week)
			if err != nil {
				return err
			}
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
				rows = append(rows, []string{
					event.Start, event.End, event.Title, event.Type, event.StudentID,
				})
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

func newBalanceCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show the account balance and per-student amounts owed",
		RunE: func(cmd *cobra.Command, args []string) error {
			balance, err := rc.client.Balance(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(balance)
			}
			rc.out.Printf("owed: %s\n", balance.Owed)
			rows := make([][]string, 0, len(balance.Students))
			for _, student := range balance.Students {
				rows = append(rows, []string{student.Name, student.Balance, student.StudentID})
			}
			rc.out.Table([]string{"STUDENT", "BALANCE", "STUDENT ID"}, rows)
			return nil
		},
	}
}

func newHistoryCommand(rc *runtime) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show the payment and charge ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			var fromDate, err = parseDate(from)
			if err != nil {
				return err
			}
			ledger, err := rc.client.History(cmd.Context(), fromDate)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(ledger)
			}
			rc.out.Printf("balance: %s\n", ledger.Balance)
			rows := make([][]string, 0, len(ledger.Entries))
			for _, entry := range ledger.Entries {
				rows = append(rows, []string{
					entry.Date, entry.Student, entry.Description, entry.Payments, entry.Charges, entry.Balance,
				})
			}
			rc.out.Table([]string{"DATE", "STUDENT", "DESCRIPTION", "PAYMENTS", "CHARGES", "BALANCE"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date for the ledger (YYYY-MM-DD)")
	return cmd
}

func newAnnouncementsCommand(rc *runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "announcements",
		Short: "List studio messages and email history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				announcement, err := rc.client.Announcement(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if rc.out.IsJSON() {
					return rc.out.JSON(announcement)
				}
				rc.out.Println(announcement.Body)
				return nil
			}
			announcements, err := rc.client.Announcements(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(announcements)
			}
			rows := make([][]string, 0, len(announcements))
			for _, announcement := range announcements {
				rows = append(rows, []string{
					announcement.ID, announcement.Sent, announcement.Subject, announcement.Category,
				})
			}
			rc.out.Table([]string{"ID", "SENT", "SUBJECT", "CATEGORY"}, rows)
			return nil
		},
	}
	cmd.Args = usageArgs(cobra.MaximumNArgs(1))
	return cmd
}

func newFilesCommand(rc *runtime) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:   "files",
		Short: "List shared files, class files, and class music",
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := rc.client.Files(cmd.Context())
			if err != nil {
				return err
			}
			if category != "" {
				filtered := files[:0]
				for _, file := range files {
					if file.Category == category {
						filtered = append(filtered, file)
					}
				}
				files = filtered
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(files)
			}
			rows := make([][]string, 0, len(files))
			for _, file := range files {
				rows = append(rows, []string{
					file.Category, file.Owner, file.Folder, file.Name, file.URL,
				})
			}
			rc.out.Table([]string{"CATEGORY", "OWNER", "FOLDER", "NAME", "URL"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "filter by category: shared, class-files, class-music")
	return cmd
}

func newAccountCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "account",
		Short: "Show the primary account contact details",
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := rc.client.Account(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(account)
			}
			rc.out.Table([]string{"KEY", "VALUE"}, [][]string{
				{"first_name", account.FirstName},
				{"last_name", account.LastName},
				{"phone", account.Phone},
				{"phone_2", account.Phone2},
				{"email", account.Email},
				{"address", account.Address},
			})
			return nil
		},
	}
}
