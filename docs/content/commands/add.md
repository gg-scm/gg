{
    "cmd_aliases": [],
    "cmd_class": "basic",
    "date": "2018-07-06 22:13:11-07:00",
    "lastmod": "2018-07-10 21:03:14-07:00",
    "title": "gg add",
    "usage": "gg add FILE [...]"
}

add the specified files on the next commit

<!--more-->

Mark files to be tracked under version control and added at the next
commit. If `add` is run on a file X and X is ignored, it will be
tracked. However, adding a directory with ignored files will not track
the ignored files.

`add` also marks merge conflicts as resolved like `git add`.
