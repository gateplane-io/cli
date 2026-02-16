package main

import (
	"fmt"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/table"

	"github.com/spf13/cobra"
)

func gatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gates",
		Aliases: []string{"g"},
		Short:   "Manage gates",
		Long:    "Discover and manage GatePlane gates",
	}

	cmd.AddCommand(
		gatesListCmd(),
		gatesInfoCmd(),
	)

	return cmd
}

func gatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "l"},
		Short:   "List all discovered gates",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			gates, err := client.DiscoverGates()
			if err != nil {
				return wrapError("discover gates", err)
			}

			// Add aliases from config
			for _, gate := range gates {
				for _, cfgGate := range cfg.Gates {
					if gate.Path == cfgGate.Path {
						gate.Alias = cfgGate.Alias
						break
					}
				}
			}

			format := getEffectiveOutputFormat()
			if format == OutputFormatJSON || format == OutputFormatYAML {
				return formatOutput(gates, format)
			}

			// Table format
			if len(gates) == 0 {
				fmt.Println("No GatePlane gates found")
			} else {
				rows := make([]table.Row, 0, len(gates))
				for _, gate := range gates {
					rows = append(rows, table.Row{
						formatGateDisplay(gate.Path),
						string(gate.Type),
						gate.Alias,
						gate.Description,
					})
				}

				table.RenderTable(table.TableOptions{
					Headers: []string{"Path", "Type", "Alias", "Description"},
					SortBy:  0,  // Sort by Path
					GroupBy: -1, // No grouping for gates list
				}, rows)
			}

			return nil
		},
	}
}

func gatesInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info [gate]",
		Aliases: []string{"i", "describe"},
		Short:   "Get detailed information about a gate",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gatePath := config.ResolveGatePath(args[0])

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			configPath := fmt.Sprintf("%s/config", gatePath)
			resp, err := client.VaultClient().Logical().Read(configPath)
			if err != nil {
				return wrapError("read gate config", err)
			}

			if resp == nil || resp.Data == nil {
				fmt.Printf("Gate: %s\n", gatePath)
				fmt.Println("No configuration data available")
				return nil
			}

			format := getEffectiveOutputFormat()
			if format == OutputFormatJSON || format == OutputFormatYAML {
				return formatOutput(resp.Data, format)
			}

			// Table format
			fmt.Printf("Gate: %s\n", gatePath)
			fmt.Println("Configuration:")
			for key, value := range resp.Data {
				fmt.Printf("  %s: %v\n", key, value)
			}

			return nil
		},
	}
}
