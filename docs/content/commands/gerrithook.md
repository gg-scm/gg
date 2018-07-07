{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "synopsis": "install or uninstall Gerrit change ID hook",
    "title": "gg gerrithook",
    "usage": "gg gerrithook [-url=URL] [ on | off ]"
}

The Gerrit change ID hook is a commit message hook which automatically
inserts a globally unique Change-Id tag in the footer of a commit
message.

gerrithook downloads the hook script from a public Gerrit server.

More details at https://gerrit-review.googlesource.com/hooks/commit-msg

## Options

<dl class="flag_list">
	<dt>-url string</dt>
	<dd>URL of hook script to download</dd>
</dl>
