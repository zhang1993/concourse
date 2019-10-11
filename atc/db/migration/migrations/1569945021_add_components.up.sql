BEGIN;

  CREATE TABLE components (
      id serial PRIMARY KEY,
      name text NOT NULL,
      interval text NOT NULL,
      last_ran timestamp WITH TIME ZONE,
      paused boolean DEFAULT FALSE
  );

  CREATE UNIQUE INDEX components_name_key ON components (name);

  INSERT INTO components(name, interval) VALUES
    ('build-tracker', '10s'),
    ('scheduler', '10s'),
    ('scanner', '1m'),
    ('checker', '10s'),
    ('build-reaper', '30s'),
    ('syslog-drainer', '30s'),
    ('build-collector', '30s'),
    ('worker-collector', '30s'),
    ('resource-cache-use-collector', '30s'),
    ('resource-cache-collector', '30s'),
    ('resource-config-collector', '30s'),
    ('resource-config-check-session', '30s'),
    ('artifact-collector', '30s'),
    ('volume-collector', '30s'),
    ('container-collector', '30s'),
    ('check-collector', '30s');

COMMIT;
