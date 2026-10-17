CREATE TABLE IF NOT EXISTS aiusage (
    connectionid varchar(26) NOT NULL,
    userid varchar(26) NOT NULL DEFAULT '',
    purpose varchar(64) NOT NULL DEFAULT '',
    day integer NOT NULL,
    requests bigint NOT NULL DEFAULT 0,
    inputtokens bigint NOT NULL DEFAULT 0,
    outputtokens bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (connectionid, userid, purpose, day)
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_aiusage_connectionid_day ON aiusage (connectionid, day);
