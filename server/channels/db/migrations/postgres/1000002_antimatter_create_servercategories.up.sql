CREATE TABLE IF NOT EXISTS servercategories (
    teamid varchar(26) NOT NULL,
    name varchar(255) NOT NULL,
    sortorder integer NOT NULL DEFAULT 0,
    locked boolean NOT NULL DEFAULT false,
    createat bigint NOT NULL DEFAULT 0,
    updateat bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (teamid, name)
);
