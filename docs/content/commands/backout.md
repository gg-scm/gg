{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-14 15:01:07-07:00",
    "lastmod": "2018-07-14 15:02:31-07:00",
    "title": "gg backout",
    "usage": "gg backout [options] [-r] REV"
}

reverse effect of an earlier commit

<!--more-->

Prepare a new commit with the effect of `REV` undone in the current
working copy. If no conflicts were encountered, it will be committed
immediately (unless `-n` is passed).

## Options

<dl class="flag_list">
	<dt>-e</dt>
	<dt>-edit</dt>
	<dd>invoke editor on commit message</dd>
	<dt>-n</dt>
	<dt>-no-commit</dt>
	<dd>do not commit</dd>
	<dt>-r rev</dt>
	<dd>revision</dd>
</dl>
