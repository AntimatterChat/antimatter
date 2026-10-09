-- morph:nontransactional
-- A team's roles are looked up by team.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usergroups_teamid ON usergroups (teamid) WHERE teamid <> '';
