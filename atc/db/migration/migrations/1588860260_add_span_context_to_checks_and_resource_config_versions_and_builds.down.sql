BEGIN;
  ALTER TABLE checks DROP COLUMN span_context;
  ALTER TABLE resource_config_versions DROP COLUMN span_context;
  ALTER TABLE builds DROP COLUMN span_context;
COMMIT;
