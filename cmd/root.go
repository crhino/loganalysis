package cmd

import "github.com/spf13/cobra"

var RootCmd = &cobra.Command{
	Use:   "loganalysis",
	Short: "Logging Analysis",
	Long:  "A CLI to analyze Cloud Foundry system logs",
}
