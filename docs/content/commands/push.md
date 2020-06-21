{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2020-06-21 12:23:51-07:00",
    "title": "gg push",
    "usage": "gg push [-f] [-r REF [...]] [--new-branch] [DST]"
}

push changes to the specified destination

<!--more-->

`gg push` pushes branches and tags to mirror the local repository in the
destination repository. It does not permit diverging commits unless `-f`
is passed. If the `-r` is not given, `gg push` will push all
branches that exist in both the local and destination repository as well as
all tags. The argument to `-r` must name a ref: it cannot be an
arbitrary commit.

When no destination repository is given, tries to use the remote specified by
the configuration value of `remote.pushDefault` or the remoted called
`origin` otherwise.

By default, `gg push` will fail instead of creating a new ref in the
destination repository. If this is desired (e.g. you are creating a new
branch), then you can pass `--new-branch` to override this check.
`-f` will also skip this check.

## Options

<dl class="flag_list">
	<dt>-new-branch</dt>
	<dd>allow pushing a new ref</dd>
	<dt>-f</dt>
	<dt>-force</dt>
	<dd>allow overwriting ref if it is not an ancestor, as long as it matches the remote-tracking branch</dd>
	<dt>-r ref</dt>
	<dd>source refs</dd>
</dl>
