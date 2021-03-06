select
  "revno" as "revno",
  "sha1sum" as "sha1sum",
  "label_name" as "name"
from
  "flat_labels"
  join "commits" using ("revno")
where
  "name" = :ref or
  substr("name", -length(:ref) - 1) = ('/' || :ref)
order by
  iif("name" = :ref, 0, 1)
limit 2;
