{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2019-02-13 09:01:29-08:00",
    "title": "gg gerrithook",
    "usage": "gg gerrithook [-url=URL] [ on | off ]"
}

install or uninstall Gerrit change ID hook

<!--more-->

The Gerrit change ID hook is a commit message hook which automatically
inserts a globally unique Change-Id tag in the footer of a commit
message.

gerrithook downloads the hook script from a public Gerrit server. gg
caches the last successfully fetched hook script for each URL in
`$XDG_CACHE_DIR/gg/gerrithook/`, so if the server is unavailable,
the local file is used. `-cached` can force using the cached file.

More details at https://gerrit-review.googlesource.com/hooks/commit-msg

## Options

<dl class="flag_list">
	<dt>-url string</dt>
	<dd>URL of hook script to download</dd>
	<dt>-cached</dt>
	<dd>Use local cache instead of downloading</dd>
</dl>
