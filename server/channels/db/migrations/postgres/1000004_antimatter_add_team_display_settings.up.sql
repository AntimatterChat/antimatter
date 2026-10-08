ALTER TABLE teams ADD COLUMN IF NOT EXISTS hiderolecolors boolean NOT NULL DEFAULT false;
ALTER TABLE teams ADD COLUMN IF NOT EXISTS requireteambadges boolean NOT NULL DEFAULT false;
