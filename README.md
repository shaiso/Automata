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

| Компонент | Порт | Описание |
|-----------|------|----------|
| **API Server** | :8080 | REST API для управления flows, runs, schedules |
| **Scheduler** | :8081 | Планировщик с leader election, создаёт runs по расписанию |
| **Orchestrator** | — | Парсит DAG, создаёт tasks, управляет выполнением |
| **Worker** | — | Выполняет tasks (HTTP, delay, transform) |
| **CLI** | — | Утилита командной строки для пользователей |

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

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.24 |
| HTTP Router | `net/http` |
| Логирование | `log/slog` |
| PostgreSQL | `pgx/v5` |
| RabbitMQ | `amqp091-go` |
| Cron | `robfig/cron/v3` |
| UUID | `google/uuid` |
| Метрики | `prometheus/client_golang` |

---

## Flow Spec

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
- [ ] Парсер FlowSpec
- [ ] DAG структура
- [ ] Go templates для данных
- [ ] Реализация шагов

### Фаза 6: Orchestrator
- [ ] Гибридный подход: Event-driven + Polling
  - Event-driven: слушает `runs.pending` из RabbitMQ (низкая latency)
  - Polling: периодически проверяет `ListPending()` (fallback при сбое MQ)
- [ ] State machine для run (PENDING → RUNNING → SUCCEEDED/FAILED)
- [ ] Управление зависимостями между tasks (DAG)

### Фаза 7: Worker
- [ ] Выполнение tasks
- [ ] Retry с backoff

### Фаза 8: CLI
- [ ] Команды для flows, runs, schedules

### Фаза 9: PR-Workflow + Sandbox
- [ ] Proposals (предложения изменений)
- [ ] Изолированное тестовое выполнение
- [ ] Сравнение результатов

### Фаза 10: E2E Testing
- [ ] Интеграционные тесты

---

## Быстрый старт

```bash
# Запуск инфраструктуры (Postgres + RabbitMQ)
make up

# Применить миграции
make migrate-up

# Запуск сервисов
make dev-api          # API на :8080
make dev-scheduler    # Scheduler на :8081
make dev-orchestrator
make dev-worker
```

---

## Команды

```bash
make up            # Запустить Postgres + RabbitMQ
make down          # Остановить инфраструктуру
make migrate-up    # Применить миграции
make db-shell      # Подключиться к БД

make build         # Собрать все сервисы
make test          # Запустить тесты
make fmt           # Форматирование кода
make vet           # Статический анализ
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
