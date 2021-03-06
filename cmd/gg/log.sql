select
  "sha1sum",
  "author",
  "author_date",
  "author_tzoffset",
  "message"
from "commits"
where "revno" = :revno
limit 1;
