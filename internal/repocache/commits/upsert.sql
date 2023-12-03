delete from "commit_parents"
where "oid" = :oid;

insert or ignore into "objects"("sha1") values (:tree);
insert or ignore into "users"("user") values (:author);
insert or ignore into "users"("user") values (:committer);

insert or replace into "commits" (
  "oid",
  "tree",
  "author",
  "author_timestamp",
  "author_tzoffset_mins",
  "committer",
  "commit_timestamp",
  "commit_tzoffset_mins",
  "message"
) values (
  :oid,
  (select "oid" from "objects" where "sha1" = :tree),
  (select "user_id" from "users" where "user" = :author),
  :author_timestamp,
  :author_tzoffset_mins,
  (select "user_id" from "users" where "user" = :committer),
  :commit_timestamp,
  :commit_tzoffset_mins,
  :message
);

insert or ignore into "objects"("sha1")
  select unhex("value")
  from json_each(:parents);

insert into "commit_parents"("oid", "n", "parent")
  select :oid, "key", (select "oid" from "objects" where "sha1" = unhex("value"))
  from json_each(:parents);
