-- Contribution-powered fleets change the meaning of shots/hits and the win
-- objective. Keep every prior ruleset row intact and begin isolated v3 Solo
-- statistics instead of reinterpreting completed contribution_targets_v2 games.
INSERT INTO ruleset_mode_stats(user_id,mode,ruleset)
SELECT id,'solo','contribution_fleet_v3' FROM users
ON CONFLICT(user_id,mode,ruleset) DO NOTHING;
