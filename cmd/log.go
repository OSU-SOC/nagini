package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	lib "github.com/OSU-SOC/nagini/lib"
)

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log [config YAML]",
	Short: "Parse a YAML file and pull logs.",
	Long: `Parse a YAML file and pull logs.`,
    Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(lib.ParseConfig(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(logCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// logCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// logCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
