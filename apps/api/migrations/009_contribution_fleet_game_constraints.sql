-- contribution_fleet_v3 introduces a deployment lifecycle state and must be
-- accepted by the games table without changing the meaning of legacy rows.
ALTER TABLE games
  DROP CONSTRAINT IF EXISTS games_ruleset_check;

ALTER TABLE games
  ADD CONSTRAINT games_ruleset_check
  CHECK (ruleset IN ('fleet_v1','contribution_targets_v2','contribution_fleet_v3'));

ALTER TABLE games
  DROP CONSTRAINT IF EXISTS games_status_check;

ALTER TABLE games
  ADD CONSTRAINT games_status_check
  CHECK (status IN ('setup','deployment','battle','complete'));
