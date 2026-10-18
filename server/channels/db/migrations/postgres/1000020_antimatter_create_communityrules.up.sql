CREATE TABLE IF NOT EXISTS communityrules (
    teamid varchar(26) PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT false,
    text text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT 1,
    updatedby varchar(26) NOT NULL DEFAULT '',
    updateat bigint NOT NULL DEFAULT 0
);
