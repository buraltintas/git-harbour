-- Gameplay pivot: contribution cells are the targets. Existing prototype
-- fleet games remain explicitly identifiable and are never reinterpreted.
ALTER TABLE games ADD COLUMN IF NOT EXISTS ruleset text;
UPDATE games SET ruleset='fleet_v1' WHERE ruleset IS NULL;
ALTER TABLE games ALTER COLUMN ruleset SET NOT NULL;
ALTER TABLE games ALTER COLUMN ruleset SET DEFAULT 'contribution_targets_v2';
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_ruleset_check;
ALTER TABLE games ADD CONSTRAINT games_ruleset_check CHECK (ruleset IN ('fleet_v1','contribution_targets_v2'));
CREATE INDEX IF NOT EXISTS games_ruleset_status_updated_idx ON games(mode,ruleset,status,updated_at DESC);

-- The column is retained so legacy rows remain readable. New target-board
-- setups store an empty array and freeze the contribution board itself.
ALTER TABLE game_players DROP CONSTRAINT IF EXISTS game_players_fleet_array;
ALTER TABLE game_players ALTER COLUMN fleet SET DEFAULT '[]'::jsonb;

-- Preserve legacy shot/result rows while making the next PvP implementation
-- capable of recording contribution reveals without ship semantics.
ALTER TABLE pvp_shots ADD COLUMN IF NOT EXISTS contribution_count integer;
ALTER TABLE pvp_shots ADD COLUMN IF NOT EXISTS contribution_level smallint;
ALTER TABLE pvp_shots ADD CONSTRAINT pvp_shots_contribution_reveal_check CHECK (
  (contribution_count IS NULL AND contribution_level IS NULL) OR
  (result IN ('hit','sunk') AND contribution_count > 0 AND contribution_level BETWEEN 1 AND 4)
) NOT VALID;

ALTER TABLE pvp_results ADD COLUMN IF NOT EXISTS target_count integer;
ALTER TABLE pvp_results ADD COLUMN IF NOT EXISTS targets_hit integer;
ALTER TABLE pvp_results ADD COLUMN IF NOT EXISTS misses integer;
ALTER TABLE pvp_results ADD CONSTRAINT pvp_results_target_stats_check CHECK (
  (target_count IS NULL AND targets_hit IS NULL AND misses IS NULL) OR
  (target_count > 0 AND targets_hit BETWEEN 0 AND target_count AND misses >= 0 AND shots=targets_hit+misses)
) NOT VALID;
