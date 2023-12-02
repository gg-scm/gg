insert into "objects" (
  "sha1",
  "type",
  "size",
  "content"
) values (
  :sha1,
  :type,
  :uncompressed_size,
  zeroblob(:compressed_size)
)
on conflict ("sha1") do
  update set
    "type" = :type,
    "size" = :uncompressed_size,
    "content" = zeroblob(:compressed_size)
  where "size" < 0 or "type" is null or "content" is null
returning "oid" as "oid";
