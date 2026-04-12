-- DRAFT — Option B unified schema for BINGE-OS-WATCH.
-- This file is intentionally outside of internal/repository/migrations/ so it
-- is not auto-applied yet. Once the Go code refactor is ready, this moves to
-- internal/repository/migrations/001_initial.sql and the legacy 001–010
-- migrations get deleted. A fresh DB then starts clean on this schema.
--
-- Conventions:
--   * Every PK is a TEXT UUID unless the table is a pure join table.
--   * Every timestamp column is INTEGER unix-seconds. NULL means unknown.
--   * Every foreign key to users(id) has ON DELETE CASCADE so deleting a
--     user cleanly wipes all of their data.
--   * Every N:1 or 1:1 reference to a catalog or library row also cascades.
--   * Indexes live next to the tables they accelerate.

-- =======================================================================
-- Identity
-- =======================================================================

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user' CHECK(role IN ('user','admin')),
    created_at  INTEGER NOT NULL
);

CREATE TABLE user_settings (
    user_id     TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    locale      TEXT NOT NULL DEFAULT 'en',
    theme       TEXT NOT NULL DEFAULT 'oled',
    region      TEXT NOT NULL DEFAULT 'NL',
    ics_token   TEXT NOT NULL DEFAULT '',
    updated_at  INTEGER NOT NULL
);

CREATE TABLE sessions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token         TEXT NOT NULL UNIQUE,
    expires_at    INTEGER NOT NULL,
    created_at    INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token);

-- =======================================================================
-- TMDB catalog — shared across all users. One row per TMDB entity.
-- Metadata sync refreshes these once regardless of how many users track
-- them, and deleting a user never touches the catalog.
-- =======================================================================

CREATE TABLE tmdb_show (
    id              TEXT PRIMARY KEY,
    tmdb_id         INTEGER NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    overview        TEXT NOT NULL DEFAULT '',
    poster_path     TEXT NOT NULL DEFAULT '',
    backdrop_path   TEXT NOT NULL DEFAULT '',
    first_air_date  INTEGER,
    genres          TEXT NOT NULL DEFAULT '',
    tmdb_status     TEXT NOT NULL DEFAULT '',
    refreshed_at    INTEGER NOT NULL
);
CREATE INDEX idx_tmdb_show_tmdb_id ON tmdb_show(tmdb_id);

CREATE TABLE tmdb_movie (
    id              TEXT PRIMARY KEY,
    tmdb_id         INTEGER NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    overview        TEXT NOT NULL DEFAULT '',
    poster_path     TEXT NOT NULL DEFAULT '',
    backdrop_path   TEXT NOT NULL DEFAULT '',
    release_date    INTEGER,
    runtime_minutes INTEGER NOT NULL DEFAULT 0,
    genres          TEXT NOT NULL DEFAULT '',
    tmdb_status     TEXT NOT NULL DEFAULT '',
    refreshed_at    INTEGER NOT NULL
);
CREATE INDEX idx_tmdb_movie_tmdb_id ON tmdb_movie(tmdb_id);

CREATE TABLE tmdb_season (
    id              TEXT PRIMARY KEY,
    show_id         TEXT NOT NULL REFERENCES tmdb_show(id) ON DELETE CASCADE,
    tmdb_season_id  INTEGER NOT NULL,
    season_number   INTEGER NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    overview        TEXT NOT NULL DEFAULT '',
    poster_path     TEXT NOT NULL DEFAULT '',
    air_date        INTEGER,
    episode_count   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(show_id, season_number)
);
CREATE INDEX idx_tmdb_season_show ON tmdb_season(show_id);

CREATE TABLE tmdb_episode (
    id               TEXT PRIMARY KEY,
    season_id        TEXT NOT NULL REFERENCES tmdb_season(id) ON DELETE CASCADE,
    tmdb_episode_id  INTEGER NOT NULL UNIQUE,
    episode_number   INTEGER NOT NULL,
    name             TEXT NOT NULL DEFAULT '',
    overview         TEXT NOT NULL DEFAULT '',
    still_path       TEXT NOT NULL DEFAULT '',
    air_date         INTEGER,
    runtime_minutes  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(season_id, episode_number)
);
CREATE INDEX idx_tmdb_episode_season ON tmdb_episode(season_id);
CREATE INDEX idx_tmdb_episode_air_date ON tmdb_episode(air_date);

-- Single source of truth for "episodes that actually count toward progress".
-- Season 0 (specials) and unaired/TBD episodes are filtered out here, so
-- every feature that touches progress can JOIN this view and the rule lives
-- in exactly one place.
CREATE VIEW aired_regular_episodes AS
SELECT e.id, e.season_id, e.tmdb_episode_id, e.episode_number, e.name,
       e.overview, e.still_path, e.air_date, e.runtime_minutes
FROM tmdb_episode e
JOIN tmdb_season s ON e.season_id = s.id
WHERE s.season_number > 0
  AND e.air_date IS NOT NULL
  AND e.air_date <= CAST(strftime('%s','now') AS INTEGER);

-- =======================================================================
-- Per-user library
-- =======================================================================

-- Unified library row. Exactly one of (show_id, movie_id) is populated,
-- enforced by the CHECK and the two partial unique indexes below.
-- manual_status = NULL means "auto-derive from watch events"; anything
-- else is the user's explicit override.
-- watched_at is a denormalized "most recent watch" cache refreshed by the
-- app on every watch_event insert/delete. Readers (sorts, dashboards)
-- stay cheap; the full rewatch history lives in watch_event.
CREATE TABLE user_library (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_type        TEXT NOT NULL CHECK(media_type IN ('movie','tv')),
    show_id           TEXT REFERENCES tmdb_show(id) ON DELETE CASCADE,
    movie_id          TEXT REFERENCES tmdb_movie(id) ON DELETE CASCADE,
    manual_status     TEXT CHECK(manual_status IS NULL
                                 OR manual_status IN ('plan_to_watch','watching','watched','on_hold','dropped')),
    watched_at        INTEGER,
    notes             TEXT NOT NULL DEFAULT '',
    release_notified  INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL,
    CHECK (
        (media_type = 'tv'    AND show_id  IS NOT NULL AND movie_id IS NULL) OR
        (media_type = 'movie' AND movie_id IS NOT NULL AND show_id  IS NULL)
    )
);
CREATE INDEX idx_user_library_user ON user_library(user_id);
CREATE INDEX idx_user_library_user_type ON user_library(user_id, media_type);
-- Partial unique indexes enforce "one library entry per user+show" and
-- "one library entry per user+movie" without choking on the NULL side.
CREATE UNIQUE INDEX idx_user_library_user_show  ON user_library(user_id, show_id)  WHERE show_id  IS NOT NULL;
CREATE UNIQUE INDEX idx_user_library_user_movie ON user_library(user_id, movie_id) WHERE movie_id IS NOT NULL;

-- Rewatches are first-class for both episodes and movies: multiple rows
-- allowed per (user, subject). "Is this watched" becomes an EXISTS check;
-- "when did I last watch it" becomes MAX(watched_at). Exactly one of
-- (episode_id, movie_id) is populated, enforced by the CHECK and the two
-- partial indexes below.
CREATE TABLE watch_event (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id  TEXT REFERENCES tmdb_episode(id) ON DELETE CASCADE,
    movie_id    TEXT REFERENCES tmdb_movie(id) ON DELETE CASCADE,
    watched_at  INTEGER NOT NULL,
    notes       TEXT NOT NULL DEFAULT '',
    CHECK (
        (episode_id IS NOT NULL AND movie_id IS NULL) OR
        (movie_id IS NOT NULL AND episode_id IS NULL)
    )
);
CREATE INDEX idx_watch_event_user_episode ON watch_event(user_id, episode_id) WHERE episode_id IS NOT NULL;
CREATE INDEX idx_watch_event_user_movie   ON watch_event(user_id, movie_id)   WHERE movie_id   IS NOT NULL;
CREATE INDEX idx_watch_event_episode ON watch_event(episode_id) WHERE episode_id IS NOT NULL;
-- Dedup re-imports while still allowing real rewatches: only the
-- (user, subject, exact watched_at) triple is unique. A genuine rewatch
-- with a different timestamp inserts cleanly; replaying the same export
-- twice silently no-ops.
CREATE UNIQUE INDEX uniq_watch_event_episode_at ON watch_event(user_id, episode_id, watched_at) WHERE episode_id IS NOT NULL;
CREATE UNIQUE INDEX uniq_watch_event_movie_at   ON watch_event(user_id, movie_id,   watched_at) WHERE movie_id   IS NOT NULL;
CREATE INDEX idx_watch_event_movie   ON watch_event(movie_id)   WHERE movie_id   IS NOT NULL;

-- =======================================================================
-- Ratings — per-subject tables so FKs actually work.
-- =======================================================================

CREATE TABLE rating_show (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    show_id     TEXT NOT NULL REFERENCES tmdb_show(id) ON DELETE CASCADE,
    score       INTEGER NOT NULL CHECK(score BETWEEN 1 AND 10),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(user_id, show_id)
);

CREATE TABLE rating_movie (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id    TEXT NOT NULL REFERENCES tmdb_movie(id) ON DELETE CASCADE,
    score       INTEGER NOT NULL CHECK(score BETWEEN 1 AND 10),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(user_id, movie_id)
);

CREATE TABLE rating_season (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    season_id   TEXT NOT NULL REFERENCES tmdb_season(id) ON DELETE CASCADE,
    score       INTEGER NOT NULL CHECK(score BETWEEN 1 AND 10),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(user_id, season_id)
);

CREATE TABLE rating_episode (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id  TEXT NOT NULL REFERENCES tmdb_episode(id) ON DELETE CASCADE,
    score       INTEGER NOT NULL CHECK(score BETWEEN 1 AND 10),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(user_id, episode_id)
);

-- =======================================================================
-- Tags
-- =======================================================================

CREATE TABLE tag (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    UNIQUE(user_id, name)
);

CREATE TABLE library_tag (
    library_id  TEXT NOT NULL REFERENCES user_library(id) ON DELETE CASCADE,
    tag_id      TEXT NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
    PRIMARY KEY (library_id, tag_id)
);
CREATE INDEX idx_library_tag_tag ON library_tag(tag_id);

-- =======================================================================
-- Keyword watches — per-user saved TMDB searches.
-- =======================================================================

CREATE TABLE keyword_watches (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    keyword      TEXT NOT NULL,
    media_types  TEXT NOT NULL DEFAULT 'movie,tv',
    created_at   INTEGER NOT NULL,
    UNIQUE(user_id, keyword)
);
CREATE INDEX idx_keyword_watches_user ON keyword_watches(user_id);

CREATE TABLE keyword_results (
    id                TEXT PRIMARY KEY,
    keyword_watch_id  TEXT NOT NULL REFERENCES keyword_watches(id) ON DELETE CASCADE,
    tmdb_id           INTEGER NOT NULL,
    media_type        TEXT NOT NULL CHECK(media_type IN ('movie','tv')),
    title             TEXT NOT NULL,
    poster_path       TEXT NOT NULL DEFAULT '',
    release_date      INTEGER,
    status            TEXT NOT NULL DEFAULT 'pending'
                        CHECK(status IN ('pending','added','dismissed')),
    created_at        INTEGER NOT NULL,
    UNIQUE(keyword_watch_id, tmdb_id, media_type)
);
CREATE INDEX idx_keyword_results_watch ON keyword_results(keyword_watch_id);

-- =======================================================================
-- Dismissed discovery recommendations
-- =======================================================================

CREATE TABLE dismissed_recommendations (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tmdb_id     INTEGER NOT NULL,
    media_type  TEXT NOT NULL CHECK(media_type IN ('movie','tv')),
    created_at  INTEGER NOT NULL,
    UNIQUE(user_id, tmdb_id, media_type)
);

-- =======================================================================
-- Library import queue — durable record of in-flight imports so a crash
-- mid-import (or a server restart) doesn't lose the user's upload. The
-- payload column holds the validated LibraryExport JSON; the row is
-- deleted on success. On startup the server scans this table and
-- re-kicks any rows that survived.
-- =======================================================================

CREATE TABLE library_import_job (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','failed')),
    error       TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    started_at  INTEGER
);
CREATE INDEX idx_library_import_job_user ON library_import_job(user_id);

-- =======================================================================
-- Webhooks + delivery log
-- =======================================================================

CREATE TABLE webhooks (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL DEFAULT '',
    service        TEXT NOT NULL DEFAULT 'generic',
    url            TEXT NOT NULL,
    events         TEXT NOT NULL DEFAULT 'status_changed',
    body_template  TEXT NOT NULL DEFAULT '',
    headers        TEXT NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL,
    UNIQUE(user_id, url)
);
CREATE INDEX idx_webhooks_user ON webhooks(user_id);

CREATE TABLE webhook_deliveries (
    id           TEXT PRIMARY KEY,
    webhook_id   TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event        TEXT NOT NULL,
    status_code  INTEGER NOT NULL DEFAULT 0,
    error        TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL
);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
