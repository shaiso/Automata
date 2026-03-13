// Package sandbox обеспечивает сбор и сравнение результатов тестовых выполнений flows.
//
// Sandbox позволяет:
//   - Собирать результаты sandbox run (Collector)
//   - Ожидать завершения sandbox run с polling (WaitForResult)
//   - Сравнивать результаты с baseline — предыдущим выполнением (CompareWithBaseline)
//   - Выполнять deep diff вложенных структур (Differ)
//
// Sandbox run создаётся через API (POST /api/v1/proposals/{id}/sandbox).
// Run получает ProposedSpec из proposal через поле spec_override,
// что позволяет тестировать предложенные изменения без создания версии flow.
//
// Ключевые типы:
//   - Collector — собирает SandboxResult из завершённого run и его tasks
//   - Differ    — рекурсивный deep diff для map[string]any (поддержка вложенных map, slice, скаляров)
package sandbox
