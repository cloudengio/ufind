// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/filewalk/asyncstat"
	"cloudeng.io/file/filewalk/localfs"
	"cloudeng.io/text/linewrap"
	"golang.org/x/term"
)

type locateCmd struct{}

type WalkerFlags struct {
	ConcurrentScans          int `subcmd:"concurrent-dir-scans,1000,number of concurrent directory scans"`
	ScanSize                 int `subcmd:"dir-scan-size,100,size of directory scans"`
	ConcurrentStats          int `subcmd:"async-stats-total,1000,max number of concurrent lstat system calls"`
	ConcurrentStatsThreshold int `subcmd:"async-stats-threshold,10,threshold at which to start issuing concurrent lstat system calls"`
}

type locateFlags struct {
	WalkerFlags
	Exclusions      flags.Repeating `subcmd:"exclude,,exclude directories matching the specified regexp patterns"`
	SameDevice      bool            `subcmd:"same-device,true,only search directories on the same device as the starting directory"`
	Prune           bool            `subcmd:"prune,false,stop search when a directory match is found"`
	FollowSoftLinks bool            `subcmd:"follow-softlinks,false,follow softlinks"`
	Long            bool            `subcmd:"l,false,show detailed information about each match"`
	Sorted          bool            `subcmd:"sorted,false,'output in sorted, depth-first order, like the find command'"`
}

func (w *WalkerFlags) Options(lf *locateFlags) (fwo []filewalk.Option, aso []asyncstat.Option, wo []walkerOption, err error) {
	if w.ConcurrentScans > 0 {
		fwo = append(fwo, filewalk.WithConcurrentScans(w.ConcurrentScans))
	}
	if w.ScanSize > 0 {
		fwo = append(fwo, filewalk.WithScanSize(w.ScanSize))
	}
	if w.ConcurrentStats > 0 {
		aso = append(aso, asyncstat.WithAsyncStats(w.ConcurrentStats))
	}
	if w.ConcurrentStatsThreshold > 0 {
		aso = append(aso, asyncstat.WithAsyncThreshold(w.ConcurrentStatsThreshold))
	}
	if lf.FollowSoftLinks {
		aso = append(aso, asyncstat.WithStat())
	} else {
		aso = append(aso, asyncstat.WithLStat())
	}
	ex, err := newExclusions(lf.Exclusions.Values)
	if err != nil {
		return
	}
	wo = append(wo,
		withFollowSoftLinks(lf.FollowSoftLinks),
		withStats(w.ConcurrentStats > 0),
		withScanSize(w.ScanSize),
		withExclusions(ex))
	return
}

var terminal_width = 80

func init() {
	if width, _, err := term.GetSize(0); err == nil {
		terminal_width = width
	}
	if terminal_width > 200 {
		terminal_width = 200
	}
}

func (lc locateCmd) explain(ctx context.Context, values interface{}, args []string) error {
	e, err := createExpr(false, []string{})
	if err != nil {
		return err
	}
	p := e.parser
	var out strings.Builder
	out.WriteString("idu commands accept boolean expressions using || && ! and ( and ) to combine any of the following operands:\n\n")

	for _, op := range p.ListOperands() {
		out.WriteString("  ")
		out.WriteString(op.Document())
		out.WriteRune('\n')
		out.WriteRune('\n')
	}

	out.WriteString(`
Note that the name operand evaluates both the name of a file or directory
within the directory that contains it as well as its full path name. The re
(regexp) operand evaluates the full path name of a file or directory.

For example 'name=bar' will match a file named 'bar' in directory '/foo',
as will 'name=/foo/bar'. Since name uses glob matching all directory
levels must be specified, i.e. 'name=/*/*/baz' is required to match
/foo/bar/baz. The re (regexp) operator can be used to match any level,
 for example 're=bar' will match '/foo/bar/baz' as will 're=bar/baz.

The dir-larger operand matches directories that contain more than the
specified number incrementally and hence entries that are encountered
before the limit is reached may not be displayed.
`)

	out.WriteString(`
The expression may span multiple arguments which are concatenated together using spaces. Operand values may be quoted using single quotes or may contain escaped characters using. For example re='a b.pdf' or or re=a\\ b.pdf\n
`)
	fmt.Println(linewrap.Block(4, terminal_width, out.String()))
	return nil
}

type visitor func(parent, name string, entry filewalk.Entry, fi *file.Info, err error)

type visit struct {
	ctx context.Context
	fs  filewalk.FS
	lf  *locateFlags
}

func (v visit) visit(parent, name string, entry filewalk.Entry, fi *file.Info, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %v\n", v.fs.Join(parent, name), err)
		return
	}
	if fi == nil || !v.lf.Long {
		fmt.Println(v.fs.Join(parent, name))
		return
	}
	xattr, err := v.fs.XAttr(v.ctx, v.fs.Join(parent, name), *fi)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %v\n", v.fs.Join(parent, name), err)
	}
	var user, group = fmt.Sprintf("%v", xattr.UID), fmt.Sprintf("%v", xattr.GID)
	if id, err := idm.LookupUser(user); err == nil {
		user = id.Username
	}
	if id, err := idm.LookupGroup(group); err == nil {
		group = id.Name
	}
	fmt.Printf("%s: %s (%v, %v)\n", v.fs.Join(parent, name), fs.FormatFileInfo(fi), user, group)
}

func (lc locateCmd) locate(ctx context.Context, values interface{}, args []string) error {
	wkfs := localfs.New()
	lf := values.(*locateFlags)
	visit := visit{fs: wkfs, ctx: ctx, lf: lf}
	return lc.locateFS(ctx, wkfs, lf, visit.visit, args)
}

func (lc locateCmd) locateFS(ctx context.Context,
	wkfs filewalk.FS,
	lf *locateFlags,
	visit visitor,
	args []string) error {
	wko, aso, wo, err := lf.WalkerFlags.Options(lf)
	if err != nil {
		return err
	}
	if lf.SameDevice {
		sd, err := newSameDevice(ctx, wkfs, args[0])
		if err != nil {
			return err
		}
		wo = append(wo, withSameDevice(sd))
	}
	stats := asyncstat.New(wkfs, aso...)
	expr, err := createExpr(lf.Prune, args[1:])
	if err != nil {
		return err
	}
	needsStat := expr.NeedsStat() || lf.Long
	if !lf.Sorted {
		return newWalker(expr, wkfs, stats, needsStat, wko, wo, visit).Walk(ctx, args[0])
	}
	dfw := newDepthFirstWalker(expr, wkfs, stats, wo, visit)
	return dfw.start(ctx, args[0])
}
