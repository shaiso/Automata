// Automata CLI — инструмент командной строки для управления
// flows, runs и schedules через HTTP API.
//
// Использование:
//
//	automata [--api-url URL] [--json] <command> <subcommand> [flags]
//
// Команды:
//
//	flow      Управление flows
//	run       Управление runs
//	schedule  Управление schedules
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/shaiso/Automata/internal/cli"
)

// version задаётся через ldflags при сборке.
var version = "dev"

func main() {
	var apiURL string
	var jsonOutput bool

	rootCmd := &cobra.Command{
		Use:           "automata",
		Short:         "Automata CLI — workflow automation tool",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	clientFn := func() *cli.Client { return cli.NewClient(apiURL) }
	outputFn := func() *cli.Output { return cli.NewOutput(jsonOutput) }

	rootCmd.AddCommand(
		cli.NewFlowCmd(clientFn, outputFn),
		cli.NewRunCmd(clientFn, outputFn),
		cli.NewScheduleCmd(clientFn, outputFn),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
