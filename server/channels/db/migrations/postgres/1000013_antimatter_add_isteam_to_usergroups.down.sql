-- Teams go back to the whole server: with a team but no IsTeam, they would be taken for roles.
UPDATE usergroups SET teamid = '' WHERE isteam;
ALTER TABLE usergroups DROP COLUMN IF EXISTS isteam;
