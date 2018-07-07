{
    "cmd_aliases": [],
    "cmd_class": "advanced",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-06 22:36:13-07:00",
    "title": "gg mail",
    "usage": "gg mail [options] [DST]"
}

creates or updates a Gerrit change

<!--more-->

## Options

<dl class="flag_list">
	<dt>-allow-dirty</dt>
	<dd>allow mailing when working copy has uncommitted changes</dd>
	<dt>-d branch</dt>
	<dt>-dest branch</dt>
	<dt>-for branch</dt>
	<dd>destination branch</dd>
	<dt>-r rev</dt>
	<dd>source revision</dd>
	<dt>-R email</dt>
	<dt>-reviewer email</dt>
	<dd>reviewer email</dd>
	<dt>-CC email</dt>
	<dt>-cc email</dt>
	<dd>emails to CC</dd>
	<dt>-notify string</dt>
	<dd>who to send email notifications to; one of &#34;none&#34;, &#34;owner&#34;, &#34;owner_reviewers&#34;, or &#34;all&#34;</dd>
	<dt>-notify-to email</dt>
	<dd>email to send notification</dd>
	<dt>-notify-cc email</dt>
	<dd>email to CC notification</dd>
	<dt>-notify-bcc email</dt>
	<dd>email to BCC notification</dd>
	<dt>-m message</dt>
	<dd>use text as comment message</dd>
	<dt>-p</dt>
	<dt>-publish-comments</dt>
	<dd>publish draft comments</dd>
</dl>
