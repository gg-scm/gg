{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg histedit",
    "usage": "gg histedit [options] [UPSTREAM]"
}

interactively edit revision history

<!--more-->

This command lets you interactively edit a linear series of commits.
When starting `histedit`, it will open your editor to plan the series
of changes you want to make. You can reorder commits, or use the
actions listed in the plan comments.

Unlike `git rebase -i`, continuing a `histedit` will automatically
amend the current commit if any changes are made. In most cases,
you do not need to run `commit --amend` yourself.

## Options

<dl class="flag_list">
	<dt>-abort</dt>
	<dd>abort an edit already in progress</dd>
	<dt>-continue</dt>
	<dd>continue an edit already in progress</dd>
	<dt>-edit-plan</dt>
	<dd>edit remaining actions list</dd>
	<dt>-exec command</dt>
	<dd>execute the shell command after each line creating a commit (can be specified multiple times)</dd>
</dl>
