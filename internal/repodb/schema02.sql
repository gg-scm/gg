CREATE TABLE "labels" (
  "name" TEXT NOT NULL
    CHECK (trim("name") <> ''),
  "revno" INTEGER NOT NULL
    REFERENCES "commits" ("revno")
      ON UPDATE CASCADE
      ON DELETE CASCADE,
  -- If a tag is present, it must reference the same commit as revno.
  "tag_id" INTEGER
    REFERENCES "tag_objects"
      ON UPDATE CASCADE
      ON DELETE SET NULL,

  PRIMARY KEY ("name", "revno" desc)
);

CREATE TABLE "label_aliases" (
  "name" TEXT PRIMARY KEY NOT NULL
    CHECK (trim("name") <> ''),
  "target" TEXT NOT NULL
    CHECK (trim("target") <> ''),

  CHECK("name" <> "target")
);
