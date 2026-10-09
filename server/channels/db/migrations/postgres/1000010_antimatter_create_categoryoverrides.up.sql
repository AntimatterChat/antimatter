CREATE TABLE IF NOT EXISTS categoryoverrides (
    teamid varchar(26) NOT NULL,
    category varchar(255) NOT NULL,
    targettype varchar(16) NOT NULL,
    targetid varchar(26) NOT NULL,
    allow text NOT NULL DEFAULT '',
    deny text NOT NULL DEFAULT '',
    updateat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (teamid, category, targettype, targetid)
);
