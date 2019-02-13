{
    "cmd_aliases": [
        "up",
        "checkout",
        "co"
    ],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2019-02-13 09:01:29-08:00",
    "title": "gg update",
    "usage": "gg update [--clean] [[-r] REV]"
}

update working directory (or switch revisions)

<!--more-->

Update the working directory to the specified revision. If no
revision is specified, update to the tip of the upstream branch if
it has the same name as the current branch or the tip of the push
branch otherwise.

If the commit is not a descendant or ancestor of the HEAD commit,
the update is aborted.

## Options

<dl class="flag_list">
	<dt>-r rev</dt>
	<dd>revision</dd>
	<dt>-clean</dt>
	<dt>-C</dt>
	<dd>discard uncommitted changes (no backup)</dd>
</dl>
