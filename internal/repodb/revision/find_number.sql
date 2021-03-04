SELECT
  "sha1sum" AS "sha1sum"
FROM "commits"
WHERE "revno" = :revno
LIMIT 1;
