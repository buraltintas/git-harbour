-- Prototype Solo results used different victory and accuracy semantics. Keep
-- an immutable archive and keep contribution-target Solo statistics in a
-- separate ruleset-keyed table. Existing counters are never mutated.
CREATE TABLE IF NOT EXISTS legacy_mode_stats (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mode text NOT NULL,
  source_ruleset text NOT NULL,
  games integer NOT NULL,
  wins integer NOT NULL,
  losses integer NOT NULL,
  rating integer NOT NULL,
  shots integer NOT NULL,
  hits integer NOT NULL,
  current_streak integer NOT NULL,
  longest_streak integer NOT NULL,
  win_shots integer NOT NULL,
  archived_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id,mode,source_ruleset)
);

INSERT INTO legacy_mode_stats(user_id,mode,source_ruleset,games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots)
SELECT user_id,mode,'fleet_v1',games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots
FROM mode_stats
WHERE mode='solo' AND games>0
ON CONFLICT(user_id,mode,source_ruleset) DO NOTHING;

CREATE TABLE IF NOT EXISTS ruleset_mode_stats (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mode text NOT NULL CHECK(mode IN ('solo','pvp')),
  ruleset text NOT NULL CHECK(ruleset IN ('contribution_targets_v2')),
  games integer NOT NULL DEFAULT 0,
  wins integer NOT NULL DEFAULT 0,
  losses integer NOT NULL DEFAULT 0,
  rating integer NOT NULL DEFAULT 1200,
  shots integer NOT NULL DEFAULT 0,
  hits integer NOT NULL DEFAULT 0,
  current_streak integer NOT NULL DEFAULT 0,
  longest_streak integer NOT NULL DEFAULT 0,
  win_shots integer NOT NULL DEFAULT 0,
  PRIMARY KEY(user_id,mode,ruleset)
);

INSERT INTO ruleset_mode_stats(user_id,mode,ruleset)
SELECT id,'solo','contribution_targets_v2' FROM users
ON CONFLICT DO NOTHING;
