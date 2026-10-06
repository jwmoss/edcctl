package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newBalanceCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Read the account balance for each student",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			balance, err := rc.client.Balance(cmd.Context())
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(balance)
			}
			rows := make([][]string, 0, len(balance.Students)+1)
			for _, student := range balance.Students {
				rows = append(rows, []string{student.StudentID, student.Name, money(student.BalanceCents)})
			}
			rows = append(rows, []string{"", "Total", money(balance.BalanceCents)})
			return rc.appRows([]string{"STUDENT ID", "NAME", "BALANCE"}, rows)
		},
	}
}

func newHistoryCommand(rc *runtime) *cobra.Command {
	var from string
	var start time.Time
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Read payments and charges, newest first",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := usageArgs(cobra.NoArgs)(cmd, args); err != nil {
				return err
			}
			var err error
			start, err = parseDate(from)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			history, err := rc.client.History(cmd.Context(), start)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(history)
			}
			rows := make([][]string, 0, len(history.Entries))
			for _, entry := range history.Entries {
				description := entry.Description
				if !rc.out.IsPlain() {
					description = truncate(description, 60)
				}
				rows = append(rows, []string{entry.Date, entry.Student, description, optionalMoney(entry.PaymentCents), optionalMoney(entry.ChargeCents), money(entry.BalanceCents)})
			}
			if !rc.out.IsPlain() {
				rc.out.Printf("Since %s; balance %s; opening balance %s\n", history.From, money(history.BalanceCents), money(history.OpeningBalanceCents))
			}
			return rc.appRows([]string{"DATE", "STUDENT", "DESCRIPTION", "PAYMENT", "CHARGE", "BALANCE"}, rows)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "first ledger date (YYYY-MM-DD); default is the portal's start date")
	return cmd
}

// money formats cents as dollars. Negative amounts are credit.
func money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

func optionalMoney(cents int64) string {
	if cents == 0 {
		return ""
	}
	return money(cents)
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
