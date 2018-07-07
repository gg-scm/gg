{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg upstream",
    "usage": "gg upstream [-b BRANCH] [REF]"
}

query or set upstream branch

<!--more-->

If no positional arguments are given, the branch's upstream branch is
printed to stdout (defaulting to the current branch if none given).

If a ref argument is given, then the branch's upstream branch
(specified by `branch.*.remote` and `branch.*.merge` configuration
settings) will be set to the given value.

## Options

<dl class="flag_list">
	<dt>-b branch</dt>
	<dd>branch to query or modify</dd>
</dl>
