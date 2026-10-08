CREATE TABLE IF NOT EXISTS channeloverrides (
    channelid varchar(26) NOT NULL,
    targettype varchar(16) NOT NULL,
    targetid varchar(26) NOT NULL,
    allow text NOT NULL DEFAULT '',
    deny text NOT NULL DEFAULT '',
    updateat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (channelid, targettype, targetid)
);
