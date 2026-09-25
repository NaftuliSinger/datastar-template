CREATE TABLE todos (
  id   INTEGER PRIMARY KEY,
  description text    NOT NULL,
  completed BOOLEAN  NOT NULL DEFAULT FALSE
);