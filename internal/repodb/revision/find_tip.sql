SELECT
  "revno" AS "revno",
  "sha1sum" AS "sha1sum"
FROM "commits"
WHERE "revno" = (SELECT MAX("revno") FROM "commits")
LIMIT 1;
