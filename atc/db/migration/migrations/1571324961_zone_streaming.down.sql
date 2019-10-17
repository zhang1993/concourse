BEGIN;
  ALTER TABLE workers DROP COLUMN baggageclaim_peer_url;
  ALTER TABLE workers DROP COLUMN zone;
COMMIT;
