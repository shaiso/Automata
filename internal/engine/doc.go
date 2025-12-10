// Package engine содержит движок выполнения flow.
//
// Включает:
//   - parser.go   — парсинг FlowSpec из JSON
//   - dag.go      — построение и обход DAG (directed acyclic graph)
//   - template.go — рендеринг Go templates ({{ .inputs.x }})
//
// Engine отвечает за понимание структуры flow и определение
// порядка выполнения шагов на основе их зависимостей.
package engine
