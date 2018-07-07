{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg rebase",
    "usage": "gg rebase [--src REV | --base REV] [--dst REV] [options]"
}

move revision (and descendants) to a different branch

<!--more-->

Rebasing will replay a set of changes on top of the destination
revision and set the current branch to the final revision.

If neither `--src` or `--base` is specified, it acts as if
`--base=@{upstream}` was specified.

## Options

<dl class="flag_list">
	<dt>-base rev</dt>
	<dd>rebase everything from branching point of specified revision</dd>
	<dt>-dst rev</dt>
	<dd>rebase onto the specified revision</dd>
	<dt>-src rev</dt>
	<dd>rebase the specified revision and descendants</dd>
	<dt>-abort</dt>
	<dd>abort an interrupted rebase</dd>
	<dt>-continue</dt>
	<dd>continue an interrupted rebase</dd>
</dl>
