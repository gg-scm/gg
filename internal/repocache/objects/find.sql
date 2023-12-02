select
  "oid" as "oid",
  "type" as "type",
  "size" as "uncompressed_size",
  length("content") as "compressed_size"
from "objects"
where
  "sha1" = :sha1 and
  "type" is not null and
  "size" >= 0 and
  "content" is not null
limit 1;
