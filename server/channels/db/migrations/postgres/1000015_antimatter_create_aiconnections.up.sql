CREATE TABLE IF NOT EXISTS aiconnections (
    id varchar(26) PRIMARY KEY,
    name varchar(64) NOT NULL,
    scope varchar(26) NOT NULL,
    provider varchar(32) NOT NULL,
    baseurl varchar(1024) NOT NULL DEFAULT '',
    apikeysealed text NOT NULL DEFAULT '',
    apikeylast4 varchar(16) NOT NULL DEFAULT '',
    extraheaderssealed text NOT NULL DEFAULT '',
    extraheadernames text NOT NULL DEFAULT '[]',
    models text NOT NULL DEFAULT '{}',
    decisionsdialect varchar(16) NOT NULL DEFAULT 'openai',
    limits text NOT NULL DEFAULT '{}',
    sharing text NOT NULL DEFAULT '{}',
    audience text NOT NULL DEFAULT '{}',
    needsattention varchar(1024) NOT NULL DEFAULT '',
    creatorid varchar(26) NOT NULL DEFAULT '',
    updaterid varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    updateat bigint NOT NULL DEFAULT 0,
    deleteat bigint NOT NULL DEFAULT 0
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_aiconnections_scope ON aiconnections (scope, deleteat);
