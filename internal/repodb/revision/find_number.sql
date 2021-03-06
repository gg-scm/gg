select
  "revno" as "revno",
  "sha1sum" as "sha1sum"
from "commits"
where "revno" = iif(:revno >= 0, :revno, (select max("revno") from "commits") + :revno + 1)
limit 1;
