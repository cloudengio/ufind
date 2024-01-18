# ufind

Ultra fast find command with flexible expressions for matching files and directories.

ufind commands accept boolean expressions using ||, &&, !, (, and ) to combine any of the following operands:

```sh
    dir-larger=<size> matches a directory size greater than or equal to <size>

    dir-smaller=<size> matches a directory size smaller than <size>

    file-larger=<size> matches a file size greater than or equal to <size>

    file-smaller=<size> matches a file size smaller than <size>

    group=<gid/groupname> matches the supplied group id or name

    iname=<glob> matches a glob pattern

    name=<glob> matches a glob pattern

    newer=<time> matches a time that is newer than the specified time in time.RFC3339, time.DateTime, time.TimeOnly or time.DateOnly formats

    re=<regexp> matches a regular expression

    type=<type> matches a file type (d, f, l, x), where d is a directory, f a regular file, l a symbolic link and x an executable regular file

    user=<uid|username> matches the supplied user id or name
```

Note that the name operand evaluates both the name of a file or directorywithin the directory that contains it as well as its full path name. The re
(regexp) operand evaluates the full path name of a file or directory.

For example 'name=bar' will match a file named 'bar' in directory '/foo',as will 'name=/foo/bar'. Since name uses glob matching all directorylevels
must be specified, i.e. 'name=/*/*/baz' is required to match/foo/bar/baz. The re (regexp) operator can be used to match any level,for example 're=bar'
will match '/foo/bar/baz' as will 're=bar/baz.

The dir-larger operand matches directories that contain more than thespecified number incrementally and hence entries that are encounteredbefore the
limit is reached may not be displayed.

The expression may span multiple arguments which are concatenated together using spaces. Operand values may be quoted using single quotes or may contain
escaped characters using. For example re='a b.pdf' or or re=a\\ b.pdf\n
