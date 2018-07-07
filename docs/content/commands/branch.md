{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg branch",
    "usage": "gg branch [-d] [-f] [-r REV] [NAME [...]]"
}

list or manage branches

<!--more-->

Branches are references to commits to help track lines of
development. Branches are unversioned and can be moved, renamed, and
deleted.

Creating or updating to a branch causes it to be marked as active.
When a commit is made, the active branch will advance to the new
commit. A plain `gg update` will also advance an active branch, if
possible. If the revision specifies a branch with an upstream, then
any new branch will use the named branch's upstream.

## Options

<dl class="flag_list">
	<dt>-d</dt>
	<dt>-delete</dt>
	<dd>delete the given branches</dd>
	<dt>-f</dt>
	<dt>-force</dt>
	<dd>force</dd>
	<dt>-r rev</dt>
	<dd>revision to place branches on</dd>
</dl>
