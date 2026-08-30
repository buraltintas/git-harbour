-- The first contribution_targets_v2 release was a one-sided history hunt in
-- which every completion counted as a win. Preserve those counters for audit,
-- then start reciprocal Solo W/L and Elo from a clean semantic baseline.
INSERT INTO legacy_mode_stats(user_id,mode,source_ruleset,games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots)
SELECT user_id,mode,'contribution_targets_v2_history_hunt',games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots
FROM ruleset_mode_stats
WHERE mode='solo' AND ruleset='contribution_targets_v2' AND games>0
ON CONFLICT(user_id,mode,source_ruleset) DO NOTHING;

UPDATE ruleset_mode_stats
SET games=0,wins=0,losses=0,rating=1200,shots=0,hits=0,current_streak=0,longest_streak=0,win_shots=0
WHERE mode='solo' AND ruleset='contribution_targets_v2';
