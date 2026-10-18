CREATE TABLE IF NOT EXISTS communityrulesacceptances (
    teamid varchar(26) NOT NULL,
    userid varchar(26) NOT NULL,
    version bigint NOT NULL,
    acceptedat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (teamid, userid, version)
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_communityrulesacceptances_userid ON communityrulesacceptances (userid);
