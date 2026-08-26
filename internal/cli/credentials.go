package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"app/backend"
)

func newCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "creds",
		Aliases: []string{"credentials"},
		Short:   "Manage Google Photos credentials",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			if err := backend.LoadConfig(configPath); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			return nil
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <auth-string>",
			Short: "Add a new credential",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := (&backend.ConfigManager{}).AddCredentials(args[0]); err != nil {
					return fmt.Errorf("adding credentials: %w", err)
				}
				fmt.Println("✓ Credentials added successfully")
				return nil
			},
		},
		&cobra.Command{
			Use:     "remove <email>",
			Aliases: []string{"rm"},
			Short:   "Remove a credential by email",
			Args:    cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := (&backend.ConfigManager{}).RemoveCredentials(args[0]); err != nil {
					return fmt.Errorf("removing credentials: %w", err)
				}
				fmt.Printf("✓ Credentials for %s removed successfully\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:     "list",
			Aliases: []string{"ls"},
			Short:   "List all credentials",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				listCredentials(cmd.Root().Name())
				return nil
			},
		},
		&cobra.Command{
			Use:     "set <email>",
			Aliases: []string{"select"},
			Short:   "Set active credential (supports partial matching)",
			Args:    cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				return selectCredential(args[0])
			},
		},
	)
	return cmd
}

func credentialEmails(config backend.Config) []string {
	emails := make([]string, 0, len(config.Account.Credentials))
	for _, cred := range config.Account.Credentials {
		params, err := backend.ParseAuthString(cred)
		if err != nil {
			emails = append(emails, "")
			continue
		}
		emails = append(emails, params.Get("Email"))
	}
	return emails
}

func listCredentials(executableName string) {
	config := (&backend.ConfigManager{}).GetConfig()
	if len(config.Account.Credentials) == 0 {
		fmt.Println("No credentials found")
		return
	}
	fmt.Println("Credentials:")
	for i, email := range credentialEmails(config) {
		if email == "" {
			fmt.Printf("  %d. [Invalid credential]\n", i+1)
			continue
		}
		marker := " "
		if email == config.Account.Selected {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, email)
	}
	if config.Account.Selected != "" {
		fmt.Printf("\n* = active\n")
	}
	fmt.Printf("\nUse '%s creds set <email>' to change active account (supports partial matching)\n", executableName)
}

func selectCredential(query string) error {
	configManager := &backend.ConfigManager{}
	emails := credentialEmails(configManager.GetConfig())

	var matched string
	var candidates []string
	for _, email := range emails {
		if email == "" {
			continue
		}
		if email == query {
			matched = email
			break
		}
		if strings.Contains(strings.ToLower(email), strings.ToLower(query)) {
			candidates = append(candidates, email)
		}
	}
	if matched == "" {
		switch len(candidates) {
		case 0:
			return fmt.Errorf("no credentials found matching '%s'", query)
		case 1:
			matched = candidates[0]
		default:
			fmt.Fprintf(os.Stderr, "Error: multiple credentials match '%s':\n", query)
			for _, email := range candidates {
				fmt.Fprintf(os.Stderr, "  - %s\n", email)
			}
			fmt.Fprintln(os.Stderr, "Please be more specific")
			return exitError{code: 1}
		}
	}

	configManager.SetSelected(matched)
	fmt.Printf("✓ Active credential set to %s\n", matched)
	return nil
}
