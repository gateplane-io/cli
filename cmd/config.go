package main

import (
	"fmt"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Manage configuration",
		Long:    "Manage GatePlane CLI configuration",
	}

	cmd.AddCommand(
		configShowCmd(),
		configSetCmd(),
		configAddAliasCmd(),
		configUseProfileCmd(),
	)

	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Aliases: []string{"get", "view"},
		Short:   "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			// Mask token for security
			displayCfg := *cfg
			if displayCfg.Vault.Token != "" {
				displayCfg.Vault.Token = "***MASKED***"
			}

			if displayCfg.Service.JWT != "" {
				displayCfg.Service.JWT = "***MASKED***"
			}

			yamlData, err := yaml.Marshal(displayCfg)
			if err != nil {
				return wrapError("marshal config", err)
			}

			fmt.Print(string(yamlData))
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set configuration values",
		Long:  "Set configuration values",
	}

	cmd.AddCommand(
		configSetVaultAddressCmd(),
		configSetDefaultGateCmd(),
		configSetOutputFormatCmd(),
	)

	return cmd
}

func configSetVaultAddressCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vault-address [address]",
		Short: "Set Vault address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SetVaultAddress(args[0]); err != nil {
				return wrapError("set vault address", err)
			}
			fmt.Printf("Vault address set to: %s\n", args[0])
			return nil
		},
	}
}

func configSetDefaultGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default-gate [gate]",
		Short: "Set default gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.SetDefaultGate(args[0]); err != nil {
				return wrapError("set default gate", err)
			}
			fmt.Printf("Default gate set to: %s\n", args[0])
			return nil
		},
	}
}

func configSetOutputFormatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "output-format [format]",
		Short: "Set default output format (table, json, yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := args[0]
			if format != "table" && format != "json" && format != "yaml" {
				return fmt.Errorf("invalid output format: %s. Must be one of: table, json, yaml", format)
			}

			cfg := config.GetConfig()
			cfg.Defaults.OutputFormat = format
			if err := config.SaveConfig(); err != nil {
				return wrapError("save config", err)
			}
			fmt.Printf("Default output format set to: %s\n", format)
			return nil
		},
	}
}

func configAddAliasCmd() *cobra.Command {
	var gateType string

	cmd := &cobra.Command{
		Use:     "add-alias [gate-path] [alias]",
		Aliases: []string{"alias"},
		Short:   "Add an alias for a gate path",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			gatePath := args[0]
			alias := args[1]

			var gt models.GateType
			switch gateType {
			case "policy", "policy-gate":
				gt = models.PolicyGate
			case "okta", "okta-group-gate":
				gt = models.OktaGroupGate
			default:
				gt = models.PolicyGate // Default
			}

			if err := config.AddGateAlias(gatePath, alias, gt); err != nil {
				return wrapError("add alias", err)
			}

			fmt.Printf("Added alias '%s' for gate '%s'\n", alias, gatePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&gateType, "type", "policy-gate", "Gate type (policy-gate, okta-group-gate)")

	return cmd
}

func configUseProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use-profile [profile]",
		Aliases: []string{"profile"},
		Short:   "Switch to a different configuration profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.UseProfile(args[0]); err != nil {
				return wrapError("use profile", err)
			}
			fmt.Printf("Switched to profile: %s\n", args[0])
			return nil
		},
	}
}
