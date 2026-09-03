-- Active v4 PvP uses persistent open harbours and server-driven asynchronous
-- defence. Legacy challenge rows and fleet_v1 results remain untouched.
CREATE TABLE IF NOT EXISTS pvp_harbours (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  period_start date NOT NULL,
  board_snapshot jsonb NOT NULL CHECK (jsonb_typeof(board_snapshot)='array' AND jsonb_array_length(board_snapshot)=70),
  deployment jsonb NOT NULL CHECK (jsonb_typeof(deployment)='array'),
  is_open boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS pvp_harbours_open_updated_idx ON pvp_harbours(updated_at DESC) WHERE is_open;

INSERT INTO ruleset_mode_stats(user_id,mode,ruleset)
SELECT id,'pvp','contribution_battleship_v4' FROM users
ON CONFLICT(user_id,mode,ruleset) DO NOTHING;

ALTER TABLE pvp_results DROP CONSTRAINT IF EXISTS pvp_results_ships_sunk_check;
ALTER TABLE pvp_results ADD CONSTRAINT pvp_results_ships_sunk_check CHECK(ships_sunk BETWEEN 0 AND 14);
