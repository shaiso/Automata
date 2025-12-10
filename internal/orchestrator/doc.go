// Package orchestrator управляет выполнением runs.
//
// Orchestrator отвечает за:
//   - Получение новых runs из очереди RabbitMQ
//   - Парсинг flow spec и построение DAG
//   - Создание tasks для шагов без зависимостей
//   - Отслеживание завершения tasks
//   - Запуск следующих шагов когда зависимости удовлетворены
//   - Финализацию run (SUCCEEDED/FAILED)
//
// Orchestrator — это "мозг" системы, который координирует выполнение.
package orchestrator
