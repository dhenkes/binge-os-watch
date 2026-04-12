-- =======================================================================
-- Durable TMDB work queue.
--
-- Any long-running TMDB fetch that the UI shouldn't wait on (adding a
-- show, warming the discovery cache, refreshing a catalog) becomes a row
-- in this table. A runner picks them up asynchronously, survives restarts
-- via a ResumeAll scan on boot, and deletes rows on success.
--
-- kind:     what the worker should do ("add_catalog", "refresh_catalog", ...)
-- payload:  opaque JSON blob — the runner interprets it based on kind
-- user_id:  nullable — some jobs (e.g. trending warm) are global
-- =======================================================================

CREATE TABLE tmdb_job (
    id          TEXT PRIMARY KEY,
    user_id     TEXT REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    payload     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','failed')),
    error       TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    started_at  INTEGER
);
CREATE INDEX idx_tmdb_job_status ON tmdb_job(status);
CREATE INDEX idx_tmdb_job_user ON tmdb_job(user_id);
