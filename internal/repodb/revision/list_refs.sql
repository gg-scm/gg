select
  "name" as "name",
  "sha1sum" as "sha1sum"
from
  "flat_labels"
  join "commits" using ("revno")
order by
  "name";
