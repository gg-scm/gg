SELECT
  "revno" AS "revno",
  "sha1sum" AS "sha1sum"
FROM "commits"
WHERE
  substr(hex("sha1sum"), 1, length(:hex_rev)) = :hex_rev AND
  "sha1sum" >= :lower_sha1sum AND
  COALESCE("sha1sum" < :upper_sha1sum, TRUE)
LIMIT 2;
