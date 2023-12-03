create table "objects" (
  "oid" integer
    primary key
    not null,
  "sha1" blob
    not null
    unique
    check (length("sha1") = 20),
  "type" text
    check ("type" is null or "type" in ('blob', 'tree', 'commit', 'tag')),
  "size" integer
    not null
    check ("size" >= -1)
    default -1,
  "content" blob
) strict;

create table "users" (
  "user_id" integer
    primary key
    not null,
  "user" text
    not null
    unique
    check (length("user") > 0)
) strict;

create table "commits" (
  "oid" integer
    primary key
    not null
    references "objects",
  "tree" integer
    not null
    references "objects",

  "author" integer
    references "users",
  "author_timestamp" integer,
  "author_tzoffset_mins" integer
    check ("author_tzoffset_mins" is null or abs("author_tzoffset_mins") < 60 * 100),

  "committer" integer
    references "users",
  "commit_timestamp" integer,
  "commit_tzoffset_mins" integer
    check ("commit_tzoffset_mins" is null or abs("commit_tzoffset_mins") < 60 * 100),

  "message" text
);

create table "commit_parents" (
  "oid" integer
    not null
    references "commits",
  "n" integer
    not null
    default 0
    check ("n" >= 0),
  "parent" integer
    not null
    references "objects",
  primary key ("oid", "n")
) strict;

create table "label_names" (
  "label_id" integer
    primary key
    not null,
  "label_name" text
    not null
    unique
    check ("label_name" regexp '\S+')
) strict;

create table "labels" (
  "oid" integer
    not null
    references "commits",
  "label_id" integer
    not null
    references "label_names",
  "label_type" integer
    not null
    default 1
    check ("label_type" in (-1, 0, 1)),
  "orig_id" integer
    not null
    references "commits",
  "value" text
    check ("value" is null or "value" regexp '\S*'),
  primary key ("oid", "label_id")
) strict;
