package domain

import (
	"time"

	"github.com/google/uuid"
)

// Schedule — расписание автоматического запуска flow.
//
// Schedule позволяет запускать flow:
// - По cron-выражению: "0 9 * * *" (каждый день в 9:00)
// - По интервалу: каждые N секунд
//
// Scheduler проверяет next_due_at и создаёт run, когда время подошло.
type Schedule struct {
	// ID — уникальный идентификатор schedule.
	ID uuid.UUID `json:"id"`

	// FlowID — ссылка на flow, который нужно запускать.
	FlowID uuid.UUID `json:"flow_id"`

	// Name — имя расписания для удобства.
	Name string `json:"name,omitempty"`

	// CronExpr — cron-выражение.
	// Формат: "минуты часы дни месяцы дни_недели"
	// Примеры:
	//   "0 9 * * *"     — каждый день в 9:00
	//   "*/5 * * * *"   — каждые 5 минут
	//   "0 0 * * 0"     — каждое воскресенье в полночь
	// Если задан CronExpr, IntervalSec игнорируется.
	CronExpr string `json:"cron_expr,omitempty"`

	// IntervalSec — интервал в секундах между запусками.
	// Используется если CronExpr не задан.
	IntervalSec int `json:"interval_sec,omitempty"`

	// Timezone — часовой пояс для вычисления времени.
	// По умолчанию: "UTC".
	// Примеры: "Europe/Moscow", "America/New_York"
	Timezone string `json:"timezone"`

	// Enabled — флаг активности расписания.
	// Если false, scheduler игнорирует это расписание.
	Enabled bool `json:"enabled"`

	// NextDueAt — время следующего запуска.
	// Scheduler создаёт run, когда now >= NextDueAt.
	// После создания run, Scheduler вычисляет новое NextDueAt.
	NextDueAt *time.Time `json:"next_due_at,omitempty"`

	// LastRunAt — время последнего запуска.
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	// LastRunID — ID последнего созданного run.
	LastRunID *uuid.UUID `json:"last_run_id,omitempty"`

	// Inputs — входные параметры для запуска flow.
	// Передаются в каждый созданный run.
	Inputs map[string]any `json:"inputs,omitempty"`

	// CreatedAt — время создания schedule.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt — время последнего обновления.
	UpdatedAt time.Time `json:"updated_at"`
}

// IsCron возвращает true, если расписание использует cron-выражение.
func (s *Schedule) IsCron() bool {
	return s.CronExpr != ""
}

// IsInterval возвращает true, если расписание использует интервал.
func (s *Schedule) IsInterval() bool {
	return s.CronExpr == "" && s.IntervalSec > 0
}

// IsDue проверяет, пора ли запускать.
func (s *Schedule) IsDue(now time.Time) bool {
	if !s.Enabled {
		return false
	}
	if s.NextDueAt == nil {
		return false
	}
	return now.After(*s.NextDueAt) || now.Equal(*s.NextDueAt)
}

// RecordRun записывает информацию о запуске.
func (s *Schedule) RecordRun(runID uuid.UUID, nextDue time.Time) {
	now := time.Now()
	s.LastRunAt = &now
	s.LastRunID = &runID
	s.NextDueAt = &nextDue
	s.UpdatedAt = now
}
