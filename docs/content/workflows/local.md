---
title: Working Locally
date: 2018-07-07T13:49:05-07:00
weight: 1
---

Whether you are working by yourself or with others, the workflow for making
changes on your local machine stays the same. This guide is primarily aimed at
those who are new to version control, but is also useful for seasoned version
control users trying to see how gg stacks up to their current tool.

<!--more-->

## Creating a Repository

To create an empty repository, you use [`gg init`][].

```shell
gg init foo
cd foo
```

This will create a new folder `foo` with a `.git` directory inside it.

[`gg init`]: {{< relref "/commands/init.md" >}}

## Adding Files

Let's say you create a file inside your new repository:

```shell
nano my-thoughts.txt
```

gg will not track `my-thoughts.txt` until you explicitly tell it to, using [`gg
add`][]:

```shell
gg add my-thoughts.txt
```

You can then save your work to the repository using [`gg commit`][]. This will
create a new [Git commit][].

```shell
gg commit -m "Started thinking"
```

Every commit has a message. A commit message should be a short, human-readable
summary of what you did in that commit. A quick internet search for "good commit
messages" includes advice on how to write a good commit message and why this is
important, especially when collaborating with others.

[`gg add`]: {{< relref "/commands/add.md" >}}
[`gg commit`]: {{< relref "/commands/commit.md" >}}
[Git commit]: https://git-scm.com/docs/gitglossary#def_commit

## Modifying Files

Let's say you add more to `my-thoughts.txt`. You can get a short summary of the
files that have changed by using [`gg status`][]:

```shell
gg status
```

This would show something like:

```
M my-thoughts.txt
```

If you want a more detailed view of your changes, you can use [`gg diff`][] (short for differences).

```shell
gg diff
```

This would show something like:

```
diff --git a/my-thoughts.txt b/my-thoughts.txt
--- a/my-thoughts.txt
+++ b/my-thoughts.txt
@@ -1,1 +1,2 @@
 Hello, World!
+I'm learning gg!
```

When you're ready to make a new commit, you run `gg commit` like before.

```shell
gg commit -m "Documented my experience in following a gg tutorial"
```

Long-time Git users will notice that we did not have to "add" the files before
committing. This is intentional: gg makes every effort to emulate Mercurial's
index-less model to reduce the number of steps in common workflows. To commit
only certain files, just pass them as arguments to `gg commit`. For more
advanced partial commits, use `git add` and `git commit` directly.

[`gg diff`]: {{< relref "/commands/diff.md" >}}
[`gg status`]: {{< relref "/commands/status.md" >}}

## Removing Files

Sometimes, you will want to remove a file that has been tracked by version
control. To do this, you run [`gg rm`][] followed by `gg commit`.

```shell
gg rm my-thoughts.txt
gg commit -m "Kept my thoughts to myself"
```

Don't worry, the file is still in your repository history. Let's see how to use
gg to go back in time.

[`gg rm`]: {{< relref "/commands/remove.md" >}}

## Examining History

The primary motivation for version control is to be able to inspect previous
revisions of files. gg includes several commands to access the repository's
data.

The first command is [`gg log`].

```shell
gg log
```

`gg log` will show you a list of all the commits you have made up to this point.
Each commit is identified by a long hexadecimal string called the [hash][]. You
can use the hash to reference the commit in other commands or when talking with
other people.

If you want to see the changes made in a single commit, you can pass the `-c`
flag to `gg diff` with a hash you found from the log.

```shell
gg diff -c a199be2
```

You can also see all the changes made since a particular commit using `-r`:

```shell
gg diff -r a199be2
```

If you want to check out what the repository looked like at a particular commit,
you can use the [`gg update`], also known as `checkout`.

```shell
gg checkout -r a199be2
```

When you're done looking, you can go back to your latest work using `checkout`.

```shell
gg checkout -r main
```

`main` is the name of the default [branch][] that was created when you ran `gg
init`. You can use the name of a branch instead of a commit hash in any gg
command that takes in a commit to indicate the latest commit on that branch.

Finally, if you realize you made a mistake (perhaps you want `my-thoughts.txt`
back), you can use `gg revert` to bring files back to their state at a given
commit, then use `gg commit` to save your work as normal.

```shell
gg revert -r a199be2 my-thoughts.txt
gg commit -m "Brought back my thoughts"
```

[branch]: https://help.github.com/articles/about-branches/
[`gg log`]: {{< relref "/commands/log.md" >}}
[`gg update`]: {{< relref "/commands/update.md" >}}
[hash]: https://git-scm.com/docs/gitglossary#def_hash

## Next Steps

If you are using gg just by yourself, this may be all you need to know. Version
control lets you keep track of how your files have changed over time, and lets
you bring back files you may have accidentally deleted. However, version control
is an important tool for being able to share these changes with others. Once
you've gotten used to the commands in this guide, take a look at the other
[workflows][] to see how to share your work with others. You may also want to
look at the [command reference][].

[command reference]: {{< sectionref "commands" >}}
[workflows]: {{< sectionref "workflows" >}}
