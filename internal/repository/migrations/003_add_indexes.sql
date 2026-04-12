-- Add missing indexes on user_id columns for per-user queries.
CREATE INDEX IF NOT EXISTS idx_rating_show_user ON rating_show(user_id);
CREATE INDEX IF NOT EXISTS idx_rating_movie_user ON rating_movie(user_id);
CREATE INDEX IF NOT EXISTS idx_rating_season_user ON rating_season(user_id);
CREATE INDEX IF NOT EXISTS idx_rating_episode_user ON rating_episode(user_id);
CREATE INDEX IF NOT EXISTS idx_dismissed_recommendations_user ON dismissed_recommendations(user_id);
CREATE INDEX IF NOT EXISTS idx_tag_user ON tag(user_id);
