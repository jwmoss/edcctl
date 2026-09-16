package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jwmoss/edcctl/internal/config"
)

func newConfigCommand(rc *runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and initialize configuration",
	}
	cmd.AddCommand(newConfigShowCommand(rc))
	cmd.AddCommand(newConfigInitCommand(rc))
	return cmd
}

func newConfigShowCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration with secrets redacted",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(rc.g.configPath)
			if err != nil {
				return err
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(cfg.Redacted())
			}
			rc.out.Table([]string{"KEY", "VALUE"}, [][]string{
				{"base_url", cfg.BaseURL},
				{"account_id", cfg.AccountID},
				{"email", cfg.Email},
				{"password", cfg.Redacted()["password"]},
				{"path", config.DefaultPath()},
			})
			return nil
		},
	}
}

func newConfigInitCommand(rc *runtime) *cobra.Command {
	var (
		baseURL       string
		accountID     string
		email         string
		passwordStdin bool
		force         bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := rc.g.configPath
			if path == "" {
				path = config.DefaultPath()
			}
			if !force && fileExists(path) {
				return fmt.Errorf("config already exists at %s; use --force to overwrite", path)
			}
			cfg := config.Default()
			if baseURL != "" {
				cfg.BaseURL = baseURL
			}
			if accountID != "" {
				cfg.AccountID = accountID
			}
			if email != "" {
				cfg.Email = email
			}
			if passwordStdin {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read password from stdin: %w", err)
				}
				cfg.Password = strings.TrimSpace(string(data))
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			rc.out.Success("config written")
			rc.out.Printf("%s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", config.DefaultBaseURL, "portal base URL")
	cmd.Flags().StringVar(&accountID, "account-id", config.DefaultAccountID, "Studio Pro account ID")
	cmd.Flags().StringVar(&email, "email", "", "portal login email")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	return cmd
}
