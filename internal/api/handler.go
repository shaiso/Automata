package api

import (
	"log/slog"

	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// Handler — главный обработчик API с зависимостями.
type Handler struct {
	flowRepo     *repo.FlowRepo
	runRepo      *repo.RunRepo
	taskRepo     *repo.TaskRepo
	scheduleRepo *repo.ScheduleRepo
	publisher    *mq.Publisher
	logger       *slog.Logger
}

// Config — конфигурация для создания Handler.
type Config struct {
	FlowRepo     *repo.FlowRepo
	RunRepo      *repo.RunRepo
	TaskRepo     *repo.TaskRepo
	ScheduleRepo *repo.ScheduleRepo
	Publisher    *mq.Publisher
	Logger       *slog.Logger
}

// NewHandler создаёт новый Handler.
func NewHandler(cfg Config) *Handler {
	return &Handler{
		flowRepo:     cfg.FlowRepo,
		runRepo:      cfg.RunRepo,
		taskRepo:     cfg.TaskRepo,
		scheduleRepo: cfg.ScheduleRepo,
		publisher:    cfg.Publisher,
		logger:       cfg.Logger,
	}
}
