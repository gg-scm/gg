{
    "cmd_aliases": [
        "id"
    ],
    "cmd_class": "basic",
    "date": "2019-02-13 09:01:29-08:00",
    "lastmod": "2019-02-13 09:01:29-08:00",
    "title": "gg identify",
    "usage": "gg identify [-r REV]"
}

identify the working directory or specified revision

<!--more-->

Print a summary of the revision or working directory if no revision
was provided. The revision's hash identifier is printed, followed by
a "+" if the working copy is being summarized and there are
uncommitted changes, a list of branches it is the tip of, and a list
of tags.

## Options

<dl class="flag_list">
	<dt>-r rev</dt>
	<dd>identify the specified revision</dd>
</dl>
