// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"

	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/filewalk/asyncstat"
)

type walker struct {
	expr  expression
	stats *asyncstat.T
	fs    filewalk.FS
	visit visitor
	walkerOptions
}

type walkerOptions struct {
	needsStat       bool
	followSoftLinks bool
	scanSize        int
	exclude         exclusions
	isSameDevice    sameDevice
}

type walkerOption func(o *walkerOptions)

func withStats(v bool) walkerOption {
	return func(wo *walkerOptions) {
		wo.needsStat = v
	}
}

func withFollowSoftLinks(v bool) walkerOption {
	return func(wo *walkerOptions) {
		wo.followSoftLinks = v
	}
}

func withScanSize(v int) walkerOption {
	return func(wo *walkerOptions) {
		wo.scanSize = v
	}
}

func withSameDevice(sd sameDevice) walkerOption {
	return func(wo *walkerOptions) {
		wo.isSameDevice = sd
	}
}

func withExclusions(ex exclusions) walkerOption {
	return func(wo *walkerOptions) {
		wo.exclude = ex
	}
}

type dirstate struct {
	numEntries int64
}

func newWalker(expr expression, fs filewalk.FS, stats *asyncstat.T, fileWalkerOpts []filewalk.Option, walkerOpts []walkerOption, visit visitor) *filewalk.Walker[dirstate] {
	w := &walker{
		expr:  expr,
		fs:    fs,
		stats: stats,
		visit: visit,
	}
	for _, opt := range walkerOpts {
		opt(&w.walkerOptions)
	}
	return filewalk.New(fs, w, fileWalkerOpts...)
}

func (w *walker) Prefix(ctx context.Context, _ *dirstate, prefix string, fi file.Info, err error) (bool, file.InfoList, error) {
	if err != nil {
		w.visit(prefix, "", filewalk.Entry{}, &fi, err)
		return true, nil, nil
	}
	if w.exclude.Match(prefix) {
		return true, nil, nil
	}
	same, err := w.isSameDevice.Match(ctx, w.fs, prefix, fi)
	if err != nil {
		w.visit(prefix, "", filewalk.Entry{}, nil, err)
		return false, nil, nil
	}
	if !same {
		return true, nil, nil
	}
	ws := withStat{
		ctx:        ctx,
		name:       fi.Name(),
		path:       prefix,
		fs:         w.fs,
		info:       fi,
		numEntries: 0, // num entries is zero now.
	}
	if w.expr.Eval(ws) {
		return false, nil, nil
	}
	return false, nil, nil
}

func (w *walker) withoutStat(ctx context.Context, state *dirstate, prefix string, contents []filewalk.Entry) (file.InfoList, error) {
	var dirs []filewalk.Entry
	for _, e := range contents {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
		wn := entryType{
			name:       e.Name,
			path:       w.fs.Join(prefix, e.Name),
			mode:       e.Type,
			numEntries: state.numEntries,
		}
		if !w.expr.Eval(wn) {
			continue
		}
		w.visit(prefix, e.Name, e, nil, nil)
	}
	children, _, err := w.stats.Process(ctx, prefix, dirs)
	if err != nil {
		w.visit(prefix, "", filewalk.Entry{}, nil, err)
	}
	return children, nil
}

func (w *walker) withStat(ctx context.Context, state *dirstate, prefix string, contents []filewalk.Entry) (file.InfoList, error) {
	children, all, err := w.stats.Process(ctx, prefix, contents)
	if err != nil {
		w.visit(prefix, "", filewalk.Entry{}, nil, err)
		return nil, nil
	}
	for _, info := range all {
		ws := withStat{
			ctx:        ctx,
			name:       info.Name(),
			path:       w.fs.Join(prefix, info.Name()),
			fs:         w.fs,
			info:       info,
			numEntries: state.numEntries,
		}
		if w.expr.Eval(ws) {
			w.visit(prefix, info.Name(),
				filewalk.Entry{Name: info.Name(), Type: info.Type()}, &info, nil)
		}
	}
	return children, nil
}

func (w *walker) Contents(ctx context.Context, state *dirstate, prefix string, contents []filewalk.Entry) (file.InfoList, error) {
	state.numEntries += int64(len(contents))
	if w.needsStat {
		return w.withStat(ctx, state, prefix, contents)
	}
	return w.withoutStat(ctx, state, prefix, contents)
}

func (w *walker) Done(ctx context.Context, _ *dirstate, prefix string, err error) error {
	if err != nil {
		w.visit(prefix, "", filewalk.Entry{}, nil, err)
		return nil
	}
	return nil
}
