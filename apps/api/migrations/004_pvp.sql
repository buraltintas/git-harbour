ALTER TABLE oauth_states ADD COLUMN IF NOT EXISTS return_path text;

ALTER TABLE games ALTER COLUMN player_start DROP NOT NULL;
ALTER TABLE games ALTER COLUMN enemy_start DROP NOT NULL;
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_status_check;
ALTER TABLE games ADD CONSTRAINT games_status_check CHECK (status IN ('setup','battle','complete'));
ALTER TABLE games ADD COLUMN IF NOT EXISTS starting_player_id uuid REFERENCES users(id);
ALTER TABLE games ADD COLUMN IF NOT EXISTS current_turn_user_id uuid REFERENCES users(id);
ALTER TABLE games ADD COLUMN IF NOT EXISTS winner_user_id uuid REFERENCES users(id);
ALTER TABLE games ADD COLUMN IF NOT EXISTS completed_at timestamptz;

ALTER TABLE challenges ADD COLUMN IF NOT EXISTS game_id uuid UNIQUE REFERENCES games(id);
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS intended_opponent_id uuid REFERENCES users(id);
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS rematch_of_game_id uuid UNIQUE REFERENCES games(id);
UPDATE challenges SET creator_id=g.player_id FROM games g WHERE challenges.creator_id IS NULL AND challenges.creator_game_id=g.id;
UPDATE challenges SET expires_at=created_at+interval '7 days' WHERE expires_at IS NULL;
ALTER TABLE challenges ALTER COLUMN creator_id SET NOT NULL;
ALTER TABLE challenges ALTER COLUMN expires_at SET NOT NULL;
ALTER TABLE challenges DROP CONSTRAINT IF EXISTS challenges_status_check;
ALTER TABLE challenges ADD CONSTRAINT challenges_status_check CHECK (status IN ('open','accepted','ready','battle','complete','cancelled','expired'));
ALTER TABLE challenges ADD CONSTRAINT challenges_distinct_opponent CHECK (creator_id IS DISTINCT FROM opponent_id);
ALTER TABLE challenges ADD CONSTRAINT challenges_distinct_invitee CHECK (creator_id IS DISTINCT FROM intended_opponent_id);
ALTER TABLE challenges ADD CONSTRAINT challenges_expiry_after_creation CHECK (expires_at > created_at);
ALTER TABLE challenges ADD CONSTRAINT challenges_code_format CHECK (length(code) BETWEEN 12 AND 64 AND code ~ '^[A-Za-z0-9_-]+$');
CREATE INDEX IF NOT EXISTS challenges_open_expiry_idx ON challenges(expires_at) WHERE status='open';
CREATE INDEX IF NOT EXISTS challenges_creator_created_idx ON challenges(creator_id,created_at DESC);
CREATE INDEX IF NOT EXISTS challenges_opponent_created_idx ON challenges(opponent_id,created_at DESC);

ALTER TABLE game_players ADD COLUMN IF NOT EXISTS period_start date;
ALTER TABLE game_players ADD COLUMN IF NOT EXISTS ready_at timestamptz;
ALTER TABLE game_players ADD CONSTRAINT game_players_board_array CHECK (jsonb_typeof(board_snapshot)='array' AND jsonb_array_length(board_snapshot)=70) NOT VALID;
ALTER TABLE game_players ADD CONSTRAINT game_players_fleet_array CHECK (jsonb_typeof(fleet)='array' AND jsonb_array_length(fleet)=5) NOT VALID;
CREATE INDEX IF NOT EXISTS game_players_user_game_idx ON game_players(user_id,game_id);

CREATE TABLE IF NOT EXISTS pvp_shots (
 id bigserial PRIMARY KEY, game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
 shooter_id uuid NOT NULL REFERENCES users(id), target_user_id uuid NOT NULL REFERENCES users(id),
 turn_number integer NOT NULL CHECK(turn_number>0), x smallint NOT NULL CHECK(x BETWEEN 0 AND 9), y smallint NOT NULL CHECK(y BETWEEN 0 AND 6),
 result text NOT NULL CHECK(result IN ('miss','hit','sunk')), ship_kind text,
 created_at timestamptz NOT NULL DEFAULT now(), CHECK(shooter_id<>target_user_id),
 UNIQUE(game_id,turn_number), UNIQUE(game_id,shooter_id,x,y)
);
CREATE INDEX IF NOT EXISTS pvp_shots_game_shooter_idx ON pvp_shots(game_id,shooter_id);

CREATE TABLE IF NOT EXISTS pvp_results (
 game_id uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE, user_id uuid NOT NULL REFERENCES users(id), opponent_id uuid NOT NULL REFERENCES users(id),
 won boolean NOT NULL, shots integer NOT NULL CHECK(shots>=0), hits integer NOT NULL CHECK(hits BETWEEN 0 AND shots), ships_sunk integer NOT NULL CHECK(ships_sunk BETWEEN 0 AND 5),
 rating_before integer NOT NULL CHECK(rating_before>=0), rating_after integer NOT NULL CHECK(rating_after>=0), rating_delta integer NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(game_id,user_id), CHECK(user_id<>opponent_id), CHECK(rating_after-rating_before=rating_delta)
);
CREATE INDEX IF NOT EXISTS pvp_results_user_created_idx ON pvp_results(user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS mode_stats_pvp_leaderboard_idx ON mode_stats(rating DESC,wins DESC,games ASC,user_id) INCLUDE(losses,shots,hits) WHERE mode='pvp' AND games>0;
