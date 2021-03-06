drop view if exists "flat_labels";
create view "flat_labels" as
  select
    "name" as "name",
    "name" as "label_name",
    max("revno") as "revno"
  from
    "labels"
  group by 1
  union all
  select
    "label_aliases"."name" as "name",
    "labels"."name" as "label_name",
    max("revno") as "revno"
  from
    "label_aliases"
    join "labels" on "label_aliases"."target" = "labels"."name"
  group by 1, 2
;
