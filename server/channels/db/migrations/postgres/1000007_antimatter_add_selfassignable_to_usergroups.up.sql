ALTER TABLE usergroups ADD COLUMN IF NOT EXISTS selfassignable boolean NOT NULL DEFAULT false;
