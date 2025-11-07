create type run_status as enum ('PENDING','RUNNING','SUCCEEDED','FAILED','CANCELLED');
create type task_status as enum ('QUEUED','RUNNING','SUCCEEDED','FAILED');

create table if not exists flows(
  id uuid primary key,
  name text not null unique,
  is_active boolean not null default true,
  created_at timestamptz not null default now()
);

create table if not exists flow_versions(
  flow_id uuid references flows(id) on delete cascade,
  version int not null,
  spec jsonb not null,
  created_at timestamptz not null default now(),
  primary key (flow_id, version)
);

create table if not exists runs(
  id uuid primary key,
  flow_id uuid not null references flows(id),
  version int not null,
  status run_status not null default 'PENDING',
  started_at timestamptz,
  finished_at timestamptz,
  error text,
  idempotency_key text,
  unique (flow_id, idempotency_key)
);

create table if not exists tasks(
  id uuid primary key,
  run_id uuid not null references runs(id) on delete cascade,
  name text not null,
  attempt int not null default 0,
  status task_status not null default 'QUEUED',
  payload jsonb,
  result_ref text,
  started_at timestamptz,
  finished_at timestamptz,
  error text
);

create table if not exists schedules(
  id uuid primary key,
  flow_id uuid not null references flows(id) on delete cascade,
  cron_expr text,
  interval_sec int,
  timezone text not null default 'UTC',
  enabled boolean not null default true,
  next_due_at timestamptz,
  last_run_at timestamptz
);

create index if not exists idx_runs_status on runs(status);
create index if not exists idx_tasks_run on tasks(run_id);
create index if not exists idx_sched_due on schedules(next_due_at);
