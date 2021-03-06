insert or replace into
  "labels" ("name", "revno", "tag_id")
values (
  :name,
  coalesce(
    (select "revno" from "commits" where "sha1sum" = :sha1sum),
    (select "revno"
      from
        "tag_objects"
        join "commits" on
          "object_type" = 'commit' and
          "object_sha1sum" = "commits"."sha1sum"
      where "tag_objects"."sha1sum" = :sha1sum)
  ),
  (select "id" from "tag_objects" where "sha1sum" = :sha1sum)
);
