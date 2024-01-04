// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"io/fs"
	"strings"
	"time"

	"cloudeng.io/cmdutil/boolexpr"
	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/matcher"
	"cloudeng.io/os/userid"
)

var (
	idm      *userid.IDManager
	uid, gid func(n, v string) boolexpr.Operand
)

func init() {
	idm = userid.NewIDManager()
	uid = func(n, v string) boolexpr.Operand {
		return matcher.NewUser(n, v, func(text string) (file.XAttr, error) {
			return matcher.ParseUsernameOrID(text, idm.LookupUser)
		})
	}

	gid = func(n, v string) boolexpr.Operand {
		return matcher.NewGroup(n, v, func(text string) (file.XAttr, error) {
			return matcher.ParseGroupnameOrID(text, idm.LookupGroup)
		})
	}
}

func createExpr(prune bool, input []string) (expression, error) {
	parser := matcher.New()
	parser.RegisterOperand("user", uid)
	parser.RegisterOperand("group", gid)

	m := strings.TrimSpace(strings.Join(input, " "))
	if len(m) == 0 {
		return expression{parser: parser}, nil
	}
	expr, err := parser.Parse(m)
	return expression{T: expr, parser: parser, isSet: true, prune: prune}, err
}

type expression struct {
	boolexpr.T
	parser *boolexpr.Parser
	isSet  bool
	prune  bool
}

func (e expression) Eval(val any) bool {
	if !e.isSet {
		return true
	}
	return e.T.Eval(val)
}

func (e expression) Prune() bool {
	return e.prune
}

type needsStat struct{}

func (needsStat) ModTime() time.Time { return time.Time{} }
func (needsStat) Mode() fs.FileMode  { return 0 }
func (needsStat) Size() int64        { return 0 }
func (needsStat) XAttr() file.XAttr  { return file.XAttr{} }

// NeedsStat determines if either of the supplied boolexpr.T's include
// operands that would require a call to fs.Stat or fs.Lstat.
func (e expression) NeedsStat() bool {
	return e.T.Needs(needsStat{})
}

type numEntries struct{}

func (numEntries) NumEntries() int64 { return 0 }

// needsNumeEntries determines if the supplied boolexpr.T's include
// operands that require reading the entire directory before evaluating
// the expression.
func (e expression) NeedsNumEntries() bool {
	return e.T.Needs(numEntries{})
}

type numEntriesType struct {
	withStat
	numEntries int64
}

func (ne numEntriesType) NumEntries() int64 {
	return ne.numEntries
}

type entryType struct {
	name, path string
	mode       fs.FileMode
	numEntries int64
}

func (wn entryType) Name() string {
	return wn.name
}

func (wn entryType) Path() string {
	return wn.path
}

func (wn entryType) Type() fs.FileMode {
	return wn.mode.Type()
}

func (wn entryType) NumEntries() int64 {
	return wn.numEntries
}

type withStat struct {
	ctx        context.Context
	name, path string
	fs         filewalk.FS
	info       file.Info
	numEntries int64
}

func (ws withStat) ModTime() time.Time {
	return ws.info.ModTime()
}

func (ws withStat) Mode() fs.FileMode {
	return ws.info.Mode()
}

func (ws withStat) Type() fs.FileMode {
	return ws.info.Mode()
}

func (ws withStat) Size() int64 {
	return ws.info.Size()
}

func (ws withStat) NumEntries() int64 {
	return ws.numEntries
}

func (ws withStat) XAttr() file.XAttr {
	xattr, _ := ws.fs.XAttr(ws.ctx, ws.path, ws.info)
	return xattr
}
