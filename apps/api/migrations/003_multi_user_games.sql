CREATE TABLE IF NOT EXISTS game_players (
  game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  side text NOT NULL CHECK (side IN ('player','opponent','ai')),
  board_snapshot jsonb NOT NULL,
  fleet jsonb NOT NULL,
  shots jsonb NOT NULL DEFAULT '[]'::jsonb,
  PRIMARY KEY (game_id, side),
  UNIQUE (game_id, user_id)
);

ALTER TABLE challenges ADD COLUMN IF NOT EXISTS creator_id uuid REFERENCES users(id);
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS expires_at timestamptz;
CREATE INDEX IF NOT EXISTS games_player_updated_idx ON games (player_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS contribution_days_user_day_idx ON contribution_days (user_id, day);

