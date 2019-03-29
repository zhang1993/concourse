BEGIN;

 ALTER TABLE worker_artifacts
    ADD COLUMN worker_resource_cache_id integer,
    ADD CONSTRAINT worker_resource_cache_id_fkey FOREIGN KEY(worker_resource_cache_id) REFERENCES worker_resource_caches("id");

  ALTER TABLE resource_cache_uses
    ADD COLUMN artifact_id integer,
    ADD CONSTRAINT worker_artifact_id_fkey FOREIGN KEY(artifact_id) REFERENCES worker_artifacts("id");

COMMIT;
