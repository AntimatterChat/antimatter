CREATE TABLE IF NOT EXISTS membertimeouts (
    teamid varchar(26) NOT NULL,
    userid varchar(26) NOT NULL,
    until bigint NOT NULL,
    reason varchar(2048) NOT NULL DEFAULT '',
    createdby varchar(26) NOT NULL DEFAULT '',
    createat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (teamid, userid)
);
