{"cmd_aliases":[],"date":"2018-07-06 22:13:11-07:00","lastmod":"2018-07-06 22:30:43-07:00","synopsis":"push changes to the specified destination","title":"gg push","usage":"gg push [-f] [-n] [-r REV] [-d REF] [--create] [DST]"}

When no destination repository is given, push uses the first non-
empty configuration value of:

1.  `branch.*.pushRemote`, if the source is a branch or is part of only
    one branch.
2.  `remote.pushDefault`.
3.  `branch.*.remote`, if the source is a branch or is part of only one
    branch.
4.  Otherwise, the remote called `origin` is used.

If `-d` is given and begins with `refs/`, then it specifies the remote
ref to update. If the argument passed to `-d` does not begin with
`refs/`, it is assumed to be a branch name (`refs/heads/<arg>`).
If `-d` is not given and the source is a ref or part of only one local
branch, then the same ref name is used. Otherwise, push exits with a
failure exit code. This differs from git, which will consult
`remote.*.push` and `push.default`. You can imagine this being the most
similar to `push.default=current`.

By default, `gg push` will fail instead of creating a new ref on the
remote. If this is desired (e.g. you are creating a new branch), then
you can pass `--create` to override this check.

## Flags

<dl class="flag_list">
	<dt>-create</dt>
	<dd>allow pushing a new ref</dd>
	<dt>-d ref</dt>
	<dt>-dest ref</dt>
	<dd>destination ref</dd>
	<dt>-f</dt>
	<dd>allow overwriting ref if it is not an ancestor, as long as it matches the remote-tracking branch</dd>
	<dt>-n</dt>
	<dt>-dry-run</dt>
	<dd>do everything except send the changes</dd>
	<dt>-r rev</dt>
	<dd>source revision</dd>
</dl>
