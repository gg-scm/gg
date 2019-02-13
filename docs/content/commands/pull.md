{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2019-02-13 09:01:29-08:00",
    "title": "gg pull",
    "usage": "gg pull [-u] [-r REF] [SOURCE]"
}

pull changes from the specified source

<!--more-->

The fetched reference is written to FETCH_HEAD.

If no source repository is given and a branch with a remote tracking
branch is currently checked out, then that remote is used. Otherwise,
the remote called "origin" is used.

If no remote reference is given and the source repository is a named
remote (like "origin"), then the remote's configured refspecs will be
fetched. (This usually means that all the remote-tracking branches
will be updated.) Any refs deleted on the remote will be pruned.

Otherwise, the source repository is assumed to be a URL. Only a single
ref will be fetched in this case and written to `FETCH_HEAD`, a
special ref name. If no remote reference is given and a branch is
currently checked out, then the branch's remote tracking branch is
used or the branch with the same name if the branch has no remote
tracking branch. Otherwise `HEAD` is used.

## Options

<dl class="flag_list">
	<dt>-r ref</dt>
	<dd>remote reference intended to be pulled</dd>
	<dt>-tags</dt>
	<dd>pull all tags from remote</dd>
	<dt>-u</dt>
	<dd>update to new head if new descendants were pulled</dd>
</dl>
