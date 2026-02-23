# Automata

Распределённая система автоматизации бизнес-процессов, предназначенная для описания, тестирования и безопасного внедрения сценариев обработки данных и интеграций с внешними сервисами.

## Возможности

- Описание и выполнение бизнес-процессов в виде сценариев (flows)
- Поддержка DAG-зависимостей между шагами
- PR-workflow для контроля изменений в сценариях
- Sandbox для тестового выполнения перед продом
- Запуск по расписанию (cron/interval)
- Retry с exponential backoff
- Dead Letter Queue (DLQ) для обработки ошибок
- Горизонтальное масштабирование воркеров

---

### Компоненты

| Компонент | Порт  | Описание |
|-----------|-------|----------|
| **API Server** | :8080 | REST API для управления flows, runs, schedules |
| **Scheduler** | :8081 | Планировщик с leader election, создаёт runs по расписанию |
| **Orchestrator** | :8083 | Парсит DAG, создаёт tasks, управляет выполнением |
| **Worker** | :8082 | Выполняет tasks (HTTP, delay, transform) |
| **CLI** | —     | Утилита командной строки для пользователей |

### Потоки данных

1. **API** → создаёт flows, runs, schedules в PostgreSQL
2. **Scheduler** → опрашивает due schedules → создаёт runs → публикует в RabbitMQ
3. **Orchestrator** → потребляет runs → парсит DAG → создаёт tasks → публикует в RabbitMQ
4. **Worker** → потребляет tasks → выполняет → публикует результат

### Принцип устойчивости

**PostgreSQL — source of truth, RabbitMQ — оптимизация.**

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│  Scheduler  │────▶│  PostgreSQL  │────▶│ Orchestrator │
│             │     │   (runs)     │     │   (polling)  │
└──────┬──────┘     └──────────────┘     └──────▲───────┘
       │                                        │
       │            ┌──────────────┐            │
       └───────────▶│   RabbitMQ   │────────────┘
                    │ (run.pending)│  event-driven
                    └──────────────┘
```

- **Нормальный режим**: RabbitMQ доставляет события мгновенно
- **При сбое MQ**: Orchestrator подхватывает runs через polling из БД
- **Гарантия**: ни один run не потеряется, даже если RabbitMQ недоступен

---

## Структура проекта

```
Automata/
├── cmd/
│   ├── automata-api/           # HTTP API сервер
│   ├── automata-scheduler/     # Планировщик задач
│   ├── automata-orchestrator/  # Оркестратор выполнения
│   ├── automata-worker/        # Воркер
│   └── automata-cli/           # CLI утилита
│
├── internal/
│   ├── domain/       # Доменные модели (Flow, Run, Task, Schedule)
│   ├── repo/         # PostgreSQL репозитории
│   ├── mq/           # RabbitMQ (connection, publisher, consumer)
│   ├── engine/       # Парсер FlowSpec, DAG, templates
│   ├── steps/        # Реализации шагов (http, delay, transform)
│   ├── scheduler/    # Логика планировщика
│   ├── orchestrator/ # Управление состоянием run
│   ├── worker/       # Выполнение tasks
│   ├── api/          # HTTP handlers, middleware, DTOs
│   ├── sandbox/      # Изолированное выполнение
│   ├── config/       # Конфигурация
│   └── telemetry/    # Логирование, метрики
│
├── migrations/       # SQL миграции
└── deploy/           # Docker Compose
```

---

## Технологии

| Компонент   | Технология |
|-------------|------------|
| Язык        | Go 1.24 |
| HTTP Router | `net/http` |
| Логирование | `log/slog` |
| PostgreSQL  | `pgx/v5` |
| RabbitMQ    | `amqp091-go` |
| Cron        | `robfig/cron/v3` |
| UUID        | `google/uuid` |
| Метрики     | `prometheus/client_golang` |
| CLI         | `github.com/spf13/cobra` |
---

## Flow Spec


Flow — это "шаблон" или "рецепт" автоматизации.
Один flow может иметь множество версий (FlowVersion).
Каждый запуск (Run) выполняет конкретную версию flow.
Flow описывается в формате JSON:

```json
{
  "name": "sync-orders",
  "inputs": {
    "source_id": { "type": "string", "required": true }
  },
  "defaults": {
    "retry": { "max_attempts": 3, "backoff": "exponential" },
    "timeout_sec": 300
  },
  "steps": [
    {
      "id": "fetch",
      "type": "http",
      "config": {
        "method": "GET",
        "url": "https://crm.example.com/orders"
      },
      "outputs": {
        "orders": "{{ .response.body.data }}"
      }
    },
    {
      "id": "save",
      "type": "http",
      "depends_on": ["fetch"],
      "config": {
        "method": "POST",
        "url": "https://erp.example.com/import",
        "body": "{{ .steps.fetch.outputs.orders }}"
      }
    }
  ]
}
```

### Типы шагов

| Тип | Описание |
|-----|----------|
| `http` | HTTP запросы к внешним API |
| `delay` | Пауза между шагами |
| `transform` | Трансформация данных |
| `parallel` | Параллельное выполнение веток |

---

## Фазы реализации

### Фаза 0: Подготовка 
- [x] Добавить зависимости: amqp091-go, google/uuid
- [x] Создать структуру internal/ пакетов
- [x] Настроить structured logging (slog)

### Фаза 1: Domain + Repository
- [x] `internal/domain/*` — доменные структуры (Flow, Run, Task, Schedule, Proposal)
- [x] `internal/repo/*` — CRUD для flows, runs, tasks, schedules, proposals

### Фаза 2: RabbitMQ
- [x] Подключение с reconnect
- [x] Publisher и Consumer
- [x] Topology (exchanges, queues)

### Фаза 3: REST API
- [x] CRUD /flows, /runs, /schedules
- [x] Версионирование flows

### Фаза 4: Scheduler
- [x] Leader election (advisory locks)
- [x] Обработка due schedules
- [x] Cron parsing (robfig/cron/v3)
- [x] Публикация run.pending в RabbitMQ

### Фаза 5: Engine + Steps
- [x] Парсер FlowSpec
- [x] DAG структура
- [x] Go templates для данных
- [x] Реализация шагов

### Фаза 6: Orchestrator
- [x] Гибридный подход: Event-driven + Polling
  - Event-driven: слушает `runs.pending` из RabbitMQ (низкая latency)
  - Polling: периодически проверяет `ListPending()` (fallback при сбое MQ)
- [x] State machine для run (PENDING → RUNNING → SUCCEEDED/FAILED)
- [x] Управление зависимостями между tasks (DAG)
- [x] RunState для управления состоянием в памяти
- [x] Восстановление состояния после рестарта

### Фаза 7: Worker
- [x] Executor interface + Registry (http, delay, transform)
- [x] HTTPExecutor (GET/POST/PUT/DELETE, headers, body, timeout, JSON parsing)
- [x] DelayExecutor (context-aware timer)
- [x] TransformExecutor (pass-through rendered payload)
- [x] Retry с exponential backoff (in-process, OnStatus для HTTP)
- [x] Гибридный подход: Consumer (tasks.ready) + Polling fallback
- [x] Graceful shutdown, publisher nil-safety
- [x] Полная точка входа cmd/automata-worker

### Фаза 8: CLI
- [x] Cobra CLI framework (github.com/spf13/cobra)
- [x] HTTP-клиент для API (Client с полным покрытием эндпоинтов)
- [x] Форматирование вывода (таблицы + --json)
- [x] Команды flow: list, create, show, update, delete, versions, publish
- [x] Команды run: list, start, show, cancel, tasks
- [x] Команды schedule: list, create, show, update, delete, enable, disable
- [x] Точка входа cmd/automata-cli с PersistentFlags (--api-url, --json)

### Фаза 9: PR-Workflow + Sandbox
- [ ] Proposals (предложения изменений)
- [ ] Изолированное тестовое выполнение
- [ ] Сравнение результатов

### Фаза 10: E2E Testing
- [ ] Интеграционные тесты

---

## Быстрый старт

```bash
make up-all       # Собрать образы и поднять всё: Postgres, RabbitMQ, миграции, 4 сервиса
make down-all     # Остановить всё и удалить контейнеры
make logs-all     # Логи всех сервисов (follow, последние 100 строк)

```

---
## CLI-команды

CLI собирается из `cmd/automata-cli/` и взаимодействует с API через HTTP.

```bash
go build -o automata ./cmd/automata-cli/
```

Глобальные флаги:
- `--api-url` — адрес API (по умолчанию `http://localhost:8080`)
- `--json` — вывод в JSON-формате

### Flows

```bash
automata flow list                          # Список всех flows
automata flow create --name "my-flow"       # Создать flow
automata flow show <ID>                     # Детали flow
automata flow update <ID> --name "new-name" # Обновить имя
automata flow update <ID> --active true     # Активировать
automata flow delete <ID>                   # Удалить
automata flow versions <ID>                 # Список версий
automata flow publish <ID> --spec-file f.json  # Опубликовать версию
```

### Runs

```bash
automata run list --flow-id <FLOW_ID>       # Список runs
automata run start <FLOW_ID>                # Запустить run
automata run start <FLOW_ID> --input "key=value" --input "k2=v2"  # С параметрами
automata run start <FLOW_ID> --sandbox      # Запуск в sandbox
automata run start <FLOW_ID> --version 2    # Конкретная версия
automata run show <RUN_ID>                  # Детали run
automata run tasks <RUN_ID>                 # Список задач в run
automata run cancel <RUN_ID>                # Отменить run
```

### Schedules

```bash
automata schedule list                      # Все расписания
automata schedule list --flow-id <ID>       # Расписания для flow
automata schedule create <FLOW_ID> --name "daily" --cron "0 9 * * *"    # По cron
automata schedule create <FLOW_ID> --name "5min" --interval 300          # По интервалу
automata schedule create <FLOW_ID> --name "tz" --cron "0 9 * * *" --timezone "Europe/Moscow"
automata schedule create <FLOW_ID> --name "with-inputs" --cron "* * * * *" --input "city=Moscow"
automata schedule show <ID>                 # Детали расписания
automata schedule update <ID> --name "new"  # Обновить
automata schedule delete <ID>               # Удалить
automata schedule enable <ID>               # Включить
automata schedule disable <ID>              # Выключить
```

---

## Модель данных

```
flows ──────────→ flow_versions (spec JSONB)
  │
  ├──────────────→ schedules (cron/interval)
  │
  └──────────────→ runs ──────────→ tasks
                     │
proposals ───────────┘ (PR-workflow)
```

### Статусы

**Run:** `PENDING` → `RUNNING` → `SUCCEEDED` | `FAILED` | `CANCELLED`

**Task:** `QUEUED` → `RUNNING` → `SUCCEEDED` | `FAILED`
