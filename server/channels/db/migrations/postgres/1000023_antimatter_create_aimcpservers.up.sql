CREATE TABLE IF NOT EXISTS aimcpservers (
    id varchar(26) PRIMARY KEY,
    name varchar(64) NOT NULL,
    slug varchar(32) NOT NULL,
    scope varchar(26) NOT NULL,
    url varchar(1024) NOT NULL,
    auth varchar(32) NOT NULL DEFAULT 'none',
    oauthclientid varchar(512) NOT NULL DEFAULT '',
    oauthclientsecretsealed text NOT NULL DEFAULT '',
    oauthscopes varchar(1024) NOT NULL DEFAULT '',
    oauthregistered boolean NOT NULL DEFAULT false,
    headerssealed text NOT NULL DEFAULT '',
    headernames text NOT NULL DEFAULT '[]',
    sharing text NOT NULL DEFAULT '{}',
    needsattention varchar(1024) NOT NULL DEFAULT '',
    creatorid varchar(26) NOT NULL DEFAULT '',
    updaterid varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    updateat bigint NOT NULL DEFAULT 0,
    deleteat bigint NOT NULL DEFAULT 0
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_aimcpservers_scope ON aimcpservers (scope, deleteat);
-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE UNIQUE INDEX IF NOT EXISTS idx_aimcpservers_slug ON aimcpservers (slug) WHERE deleteat = 0;
