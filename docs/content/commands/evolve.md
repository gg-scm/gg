{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg evolve",
    "usage": "gg evolve [-l] [-d DST]"
}

sync with Gerrit changes in upstream

<!--more-->

evolve compares HEAD with the ancestors of the given destination. If
evolve finds any ancestors of the destination have the same Gerrit
change ID as diverging ancestors of HEAD, it rebases the descendants
of the latest shared change onto the corresponding commit in the
destination.

## Options

<dl class="flag_list">
	<dt>-d ref</dt>
	<dt>-dst ref</dt>
	<dd>ref to compare with (defaults to upstream)</dd>
	<dt>-l</dt>
	<dt>-list</dt>
	<dd>list commits with match change IDs</dd>
</dl>
