delete from "labels"
where "name" = :name and "revno" > (select "revno" from "commits" where "sha1sum" = :sha1sum);
