BEGIN;
  ALTER TABLE checks ADD COLUMN span_context jsonb;
  ALTER TABLE resource_config_versions ADD COLUMN span_context jsonb;
  ALTER TABLE builds ADD COLUMN span_context jsonb;
COMMIT;
