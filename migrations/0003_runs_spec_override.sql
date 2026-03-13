-- Миграция 0003: Добавляем spec_override для sandbox runs
-- Позволяет sandbox run использовать ProposedSpec из proposal
-- вместо загрузки версии из flow_versions.

ALTER TABLE runs ADD COLUMN IF NOT EXISTS spec_override jsonb;
