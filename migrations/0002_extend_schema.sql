-- Миграция 0002: Расширение схемы для полной функциональности

-- Добавляем недостающие поля в runs
ALTER TABLE runs ADD COLUMN IF NOT EXISTS inputs jsonb;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS is_sandbox boolean NOT NULL DEFAULT false;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

-- Добавляем недостающие поля в tasks
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS step_id text;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS type text;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS outputs jsonb;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

-- Обновляем tasks: step_id = name для существующих записей
UPDATE tasks SET step_id = name WHERE step_id IS NULL;
ALTER TABLE tasks ALTER COLUMN step_id SET NOT NULL;

-- Добавляем недостающие поля в schedules
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS name text;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS last_run_id uuid REFERENCES runs(id);
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS inputs jsonb;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- Создаём тип для статуса proposal
DO $$ BEGIN
    CREATE TYPE proposal_status AS ENUM (
        'DRAFT', 'PENDING_REVIEW', 'APPROVED', 'REJECTED', 'APPLIED'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Создаём таблицу proposals для PR-workflow
CREATE TABLE IF NOT EXISTS proposals (
    id uuid PRIMARY KEY,
    flow_id uuid NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    base_version int,
    proposed_spec jsonb NOT NULL,
    status proposal_status NOT NULL DEFAULT 'DRAFT',
    title text,
    description text,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    -- Review fields
    reviewed_by text,
    reviewed_at timestamptz,
    review_comment text,

    -- Sandbox fields
    sandbox_run_id uuid REFERENCES runs(id),
    sandbox_result jsonb,

    -- Applied fields
    applied_version int,
    applied_at timestamptz
);

-- Индексы для proposals
CREATE INDEX IF NOT EXISTS idx_proposals_flow ON proposals(flow_id);
CREATE INDEX IF NOT EXISTS idx_proposals_status ON proposals(status);

-- Индекс для sandbox runs
CREATE INDEX IF NOT EXISTS idx_runs_sandbox ON runs(is_sandbox) WHERE is_sandbox = true;
