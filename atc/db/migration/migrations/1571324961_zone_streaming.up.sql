BEGIN;
  ALTER TABLE workers ADD COLUMN baggageclaim_peer_url text DEFAULT ''::text;
  ALTER TABLE workers ADD COLUMN zone text DEFAULT ''::text;
COMMIT;
