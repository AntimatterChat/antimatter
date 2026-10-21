CREATE TABLE IF NOT EXISTS userfocus (
    userid varchar(26) PRIMARY KEY,
    startat bigint NOT NULL,
    endat bigint NOT NULL,
    reason varchar(400) NOT NULL DEFAULT '',
    endedat bigint NOT NULL DEFAULT 0,
    prevstatus varchar(32) NOT NULL DEFAULT '',
    prevmanual boolean NOT NULL DEFAULT false,
    prevdndendtime bigint NOT NULL DEFAULT 0,
    prevcustomstatus text NOT NULL DEFAULT '',
    focuscustomstatus text NOT NULL DEFAULT ''
);
