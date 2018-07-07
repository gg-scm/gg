{"cmd_aliases":[],"date":"2018-07-06 22:13:11-07:00","lastmod":"2018-07-06 22:36:13-07:00","synopsis":"pull changes from the specified source","title":"gg pull","usage":"gg pull [-u] [-r REF] [SOURCE]"}

The fetched reference is written to FETCH_HEAD.

If no source repository is given and a branch with a remote tracking
branch is currently checked out, then that remote is used. Otherwise,
the remote called "origin" is used.

If no remote reference is given and a branch is currently checked out,
then the branch's remote tracking branch is used or the branch with
the same name if the branch has no remote tracking branch. Otherwise,
"HEAD" is used.

## Options

<dl class="flag_list">
	<dt>-r ref</dt>
	<dd>remote reference intended to be pulled</dd>
	<dt>-tags</dt>
	<dd>pull all tags from remote</dd>
	<dt>-u</dt>
	<dd>update to new head if new descendants were pulled</dd>
</dl>
