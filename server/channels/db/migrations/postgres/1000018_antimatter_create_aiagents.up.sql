CREATE TABLE IF NOT EXISTS aiagents (
    userid varchar(26) PRIMARY KEY,
    scope varchar(26) NOT NULL,
    external boolean NOT NULL DEFAULT false,
    connectionid varchar(26) NOT NULL DEFAULT '',
    model varchar(256) NOT NULL DEFAULT '',
    systemprompt text NOT NULL DEFAULT '',
    tools text NOT NULL DEFAULT '{}',
    reasoning text NOT NULL DEFAULT '{}',
    maxtoolturns integer NOT NULL DEFAULT 0,
    access text NOT NULL DEFAULT '{}',
    needsattention varchar(1024) NOT NULL DEFAULT '',
    creatorid varchar(26) NOT NULL DEFAULT '',
    updaterid varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    updateat bigint NOT NULL DEFAULT 0,
    deleteat bigint NOT NULL DEFAULT 0
);

-- concurrentIndex:ignore the table is created just above: it's empty, there's nothing to block
CREATE INDEX IF NOT EXISTS idx_aiagents_scope ON aiagents (scope, deleteat);
