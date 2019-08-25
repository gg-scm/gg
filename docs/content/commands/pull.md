{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2019-08-25 08:20:19-07:00",
    "title": "gg pull",
    "usage": "gg pull [-u] [-r REV [...]] [SOURCE]"
}

pull changes from the specified source

<!--more-->

Local branches with the same name as a remote branch will be
fast-forwarded if possible.

If no source repository is given and a branch with an upstream branch
is currently checked out, then the upstream's remote is used.
Otherwise, the remote called "origin" is used. If the source repository
is not a named remote, then the branches will be saved under
`refs/ggpull/`.

If no revisions are specified, then all the remote's branches and tags
will be fetched. If the source is a named remote, then its remote
tracking branches will be pruned.

## Options

<dl class="flag_list">
	<dt>-r ref</dt>
	<dd>refs to pull</dd>
	<dt>-u</dt>
	<dd>update to new head if new descendants were pulled</dd>
</dl>
