-- Restore the familiar Battleship turn model for Solo while preserving all
-- v3 games and statistics as immutable historical records.
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_ruleset_check;
ALTER TABLE games ADD CONSTRAINT games_ruleset_check CHECK (
  ruleset IN ('fleet_v1','contribution_targets_v2','contribution_fleet_v3','contribution_battleship_v4')
);

ALTER TABLE ruleset_mode_stats DROP CONSTRAINT IF EXISTS ruleset_mode_stats_ruleset_check;
ALTER TABLE ruleset_mode_stats ADD CONSTRAINT ruleset_mode_stats_ruleset_check CHECK (
  ruleset IN ('contribution_targets_v2','contribution_fleet_v3','contribution_battleship_v4')
);

INSERT INTO ruleset_mode_stats(user_id,mode,ruleset)
SELECT id,'solo','contribution_battleship_v4' FROM users
ON CONFLICT(user_id,mode,ruleset) DO NOTHING;
