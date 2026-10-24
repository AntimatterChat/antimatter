CREATE TABLE IF NOT EXISTS teaminvitelinks (
    id varchar(26) PRIMARY KEY,
    teamid varchar(26) NOT NULL,
    creatorid varchar(26) NOT NULL,
    createat bigint NOT NULL DEFAULT 0,
    expireat bigint NOT NULL DEFAULT 0,
    maxuses integer NOT NULL DEFAULT 0,
    uses integer NOT NULL DEFAULT 0
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_teaminvitelinks_teamid ON teaminvitelinks (teamid);
-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_teaminvitelinks_creatorid ON teaminvitelinks (creatorid);
