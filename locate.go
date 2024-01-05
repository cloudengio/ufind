// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/filewalk/asyncstat"
	"cloudeng.io/file/filewalk/localfs"
	"cloudeng.io/text/linewrap"
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
	Prune           bool `subcmd:"prune,false,stop search when a directory match is found"`
	FollowSoftLinks bool `subcmd:"follow-softlinks,false,follow softlinks"`
}

func (w *WalkerFlags) Options(followSoftLinks bool) (fwo []filewalk.Option, aso []asyncstat.Option) {
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
	if followSoftLinks {
		aso = append(aso, asyncstat.WithStat())
	} else {
		aso = append(aso, asyncstat.WithLStat())
	}
	return
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
`)

	out.WriteString(`
The expression may span multiple arguments which are concatenated together using spaces. Operand values may be quoted using single quotes or may contain escaped characters using. For example re='a b.pdf' or or re=a\\ b.pdf\n
`)
	fmt.Println(linewrap.Block(4, 80, out.String()))
	return nil
}

type visitor func(parent, name string, entry filewalk.Entry, fi *file.Info, err error)

type visit struct {
	ctx context.Context
	fs  filewalk.FS
}

func (v visit) visit(parent, name string, entry filewalk.Entry, fi *file.Info, err error) {
	fmt.Println(v.fs.Join(parent, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %v\n", v.fs.Join(parent, name), err)
	}
	if fi == nil {
		return
	}
	xattr, err := v.fs.XAttr(v.ctx, v.fs.Join(parent, name), *fi)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %v\n", v.fs.Join(parent, name), err)
	}
	fmt.Printf("  %v\n", xattr)
}

func (lc locateCmd) locate(ctx context.Context, values interface{}, args []string) error {
	wkfs := localfs.New()
	visit := visit{fs: wkfs, ctx: ctx}
	return lc.locateFS(ctx, wkfs, values.(*locateFlags), visit.visit, args)
}

func (lc locateCmd) locateFS(ctx context.Context,
	wkfs filewalk.FS,
	lf *locateFlags,
	visit visitor,
	args []string) error {
	wko, aso := lf.WalkerFlags.Options(lf.FollowSoftLinks)
	stats := asyncstat.New(wkfs, aso...)
	expr, err := createExpr(lf.Prune, args[1:])
	if err != nil {
		return err
	}
	needsStat := expr.NeedsStat()
	needsStat = true
	return newWalker(expr, wkfs, stats, needsStat, wko, visit).Walk(ctx, args[0])
}
