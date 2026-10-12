-- Antimatter teams belong to one community (team), as roles do: IsTeam tells them apart from roles. The teams of the
-- whole server get their community from an app migration.
ALTER TABLE usergroups ADD COLUMN IF NOT EXISTS isteam boolean NOT NULL DEFAULT false;
