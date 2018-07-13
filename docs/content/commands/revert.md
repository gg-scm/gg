{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-13 08:07:56-07:00",
    "title": "gg revert",
    "usage": "gg revert [-r REV] [--all] [--no-backup] [FILE [...]]"
}

restore files to their checkout state

<!--more-->

With no revision specified, revert the specified files or directories
to the contents they had at HEAD.

Modified files are saved with a .orig suffix before reverting. To
disable these backups, use `--no-backup`.

## Options

<dl class="flag_list">
	<dt>-all</dt>
	<dd>revert all changes when no arguments given</dd>
	<dt>-C</dt>
	<dt>-no-backup</dt>
	<dd>do not save backup copies of files</dd>
	<dt>-r rev</dt>
	<dd>revert to specified revision</dd>
</dl>
