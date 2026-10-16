CREATE TABLE IF NOT EXISTS ontopposts (
    postid varchar(26) PRIMARY KEY,
    channelid varchar(26) NOT NULL,
    createdby varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    expiresat bigint NOT NULL DEFAULT 0
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_ontopposts_channelid ON ontopposts (channelid);
