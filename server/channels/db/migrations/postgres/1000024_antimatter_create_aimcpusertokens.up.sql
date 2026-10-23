CREATE TABLE IF NOT EXISTS aimcpusertokens (
    serverid varchar(26) NOT NULL,
    userid varchar(26) NOT NULL,
    tokensealed text NOT NULL,
    expiresat bigint NOT NULL DEFAULT 0,
    createat bigint NOT NULL DEFAULT 0,
    updateat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (serverid, userid)
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_aimcpusertokens_userid ON aimcpusertokens (userid);
