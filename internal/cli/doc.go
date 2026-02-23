// Package cli реализует инструмент командной строки Automata.
//
// # Обзор
//
// CLI — клиентская утилита для взаимодействия с Automata API.
// Работает через HTTP, не импортирует внутренние пакеты системы.
// CLI используется для управления flows, runs и schedules.
//
// # Ключевые компоненты
//
// ## Client
//
// HTTP-клиент для Automata API. Инкапсулирует все HTTP-запросы,
// парсинг ответов (DataResponse, ListResponse, ErrorResponse)
// и обработку ошибок.
//
//	client := cli.NewClient("http://localhost:8080")
//	flows, err := client.ListFlows()
//
// ## Output
//
// Форматирование вывода. Поддерживает два режима:
//   - Таблицы (text/tabwriter) — по умолчанию
//   - JSON (json.MarshalIndent) — с флагом --json
//
// Данные выводятся в stdout, сообщения (Success/Error) — в stderr.
// Это позволяет использовать pipe: automata flow list --json | jq .
//
// ## Commands
//
// Cobra-команды организованы по ресурсам:
//   - flow: list, create, show, update, delete, versions, publish
//   - run: list, start, show, cancel, tasks
//   - schedule: list, create, show, update, delete, enable, disable
//
// Каждая группа создаётся через фабричную функцию (NewFlowCmd и т.д.),
// принимающую clientFn и outputFn — замыкания для ленивого создания
// Client и Output после парсинга PersistentFlags.
package cli
