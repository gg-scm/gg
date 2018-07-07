{
    "cmd_aliases": [
        "ci"
    ],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-07 13:44:58-07:00",
    "title": "gg commit",
    "usage": "gg commit [--amend] [-m MSG] [FILE [...]]"
}

commit the specified files or all outstanding changes

<!--more-->

Commits changes to the given files into the repository. If no files
are given, then all changes reported by `gg status` will be
committed.

Unlike Git, gg does not require you to stage your changes into the
index. This approximates the behavior of `git commit -a`, but
this command will only change the index if the commit succeeds.

## Options

<dl class="flag_list">
	<dt>-amend</dt>
	<dd>amend the parent of the working directory</dd>
	<dt>-m message</dt>
	<dd>use text as commit message</dd>
</dl>
