// Automata CLI — утилита командной строки.
//
// Команды:
//   automata flow list|create|show|delete
//   automata run list|start|show|cancel
//   automata schedule list|create|enable|disable|delete
//   automata proposal create|test|apply
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// TODO: реализовать команды
	// - Парсинг аргументов (можно использовать flag или cobra)
	// - HTTP клиент для обращения к API
	// - Форматированный вывод результатов

	fmt.Println("automata cli - not implemented yet")
	fmt.Printf("args: %v\n", os.Args[1:])
}

func printUsage() {
	fmt.Println(`Automata CLI - управление workflow automation

Использование:
  automata <command> <subcommand> [options]

Команды:
  flow      Управление flows
  run       Управление runs
  schedule  Управление schedules
  proposal  Управление proposals (PR-workflow)

Примеры:
  automata flow list
  automata flow create -f flow.json
  automata run start my-flow
  automata run show abc123`)
}
