CREATE TABLE IF NOT EXISTS memberbans (
    teamid varchar(26) NOT NULL,
    userid varchar(26) NOT NULL,
    reason varchar(2048) NOT NULL DEFAULT '',
    createdby varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    expireat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (teamid, userid)
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_memberbans_userid ON memberbans (userid);
