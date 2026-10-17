CREATE TABLE IF NOT EXISTS agentactivity (
    userid varchar(26) PRIMARY KEY,
    state varchar(16) NOT NULL DEFAULT 'idle',
    text varchar(1024) NOT NULL DEFAULT '',
    threadpostid varchar(26) NOT NULL DEFAULT '',
    updateat bigint NOT NULL DEFAULT 0,
    expiresat bigint NOT NULL DEFAULT 0
);
