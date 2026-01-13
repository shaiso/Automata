package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/shaiso/Automata/internal/domain"
)

// cronParser — парсер cron-выражений.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CalculateNextDue вычисляет следующее время выполнения для schedule.
// Для интервалов просто добавляет IntervalSec к текущему времени.

// Учитывает timezone schedule.
func CalculateNextDue(sched *domain.Schedule, from time.Time) (time.Time, error) {
	// Загружаем timezone
	loc, err := time.LoadLocation(sched.Timezone)
	if err != nil {
		// Fallback на UTC если timezone невалидный
		loc = time.UTC
	}

	// Конвертируем from в нужный timezone
	fromInTz := from.In(loc)

	if sched.IsCron() {
		return calculateNextCron(sched.CronExpr, fromInTz)
	}

	if sched.IsInterval() {
		return calculateNextInterval(sched.IntervalSec, fromInTz), nil
	}

	// Ни cron, ни interval — schedule некорректный
	return time.Time{}, fmt.Errorf("schedule has neither cron_expr nor interval_sec")
}

// calculateNextCron вычисляет следующее время по cron-выражению.
func calculateNextCron(cronExpr string, from time.Time) (time.Time, error) {
	schedule, err := cronParser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron expression %q: %w", cronExpr, err)
	}

	next := schedule.Next(from)
	return next.UTC(), nil // возвращаем в UTC для хранения в БД
}

// calculateNextInterval вычисляет следующее время по интервалу.
func calculateNextInterval(intervalSec int, from time.Time) time.Time {
	next := from.Add(time.Duration(intervalSec) * time.Second)
	return next.UTC() // возвращаем в UTC для хранения в БД
}

// ValidateCronExpr проверяет валидность cron-выражения.
func ValidateCronExpr(cronExpr string) error {
	_, err := cronParser.Parse(cronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}
	return nil
}

// CalculateInitialNextDue вычисляет первое время выполнения для нового schedule.
// Используется при создании schedule через API.
func CalculateInitialNextDue(sched *domain.Schedule) (time.Time, error) {
	return CalculateNextDue(sched, time.Now())
}
