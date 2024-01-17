// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"

	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/filewalk/asyncstat"
)

type depthFirst struct {
	expr  expression
	stats *asyncstat.T
	fs    filewalk.FS
	visit visitor
	walkerOptions
}

func newDepthFirstWalker(expr expression, fs filewalk.FS, stats *asyncstat.T, walkerOpts []walkerOption, visit visitor) *depthFirst {
	w := &depthFirst{
		expr:  expr,
		fs:    fs,
		stats: stats,
		visit: visit,
	}
	for _, opt := range walkerOpts {
		opt(&w.walkerOptions)
	}
	return w
}

func (d *depthFirst) start(ctx context.Context, start string) error {
	statFn := d.fs.Lstat
	if d.followSoftLinks {
		statFn = d.fs.Stat
	}
	info, err := statFn(ctx, start)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		entry := filewalk.Entry{
			Name: d.fs.Base(start),
			Type: info.Mode(),
		}
		d.visit(start, "", entry, &info, nil)
		return nil
	}
	_, err = d.handleDir(ctx, start, info)
	return err
}

func (d *depthFirst) handleDir(ctx context.Context, dirName string, dirInfo file.Info) (bool, error) {
	if d.exclude.Match(dirName) {
		fmt.Printf("EXCLUDE.... %v\n", dirName)
		return true, nil
	}
	same, err := d.isSameDevice.Match(ctx, d.fs, dirName, dirInfo)
	if err != nil {
		d.visit(dirName, "", filewalk.Entry{}, nil, err)
		return false, nil
	}
	if !same {
		return true, nil
	}
	ws := withStat{
		ctx:        ctx,
		name:       dirInfo.Name(),
		path:       dirName,
		fs:         d.fs,
		info:       dirInfo,
		numEntries: 0, // num entries is zero now.
	}
	if d.expr.Prune() && d.expr.Eval(ws) {
		return true, nil
	}

	sc := d.fs.LevelScanner(ws.path)
	numEntries := int64(0)
	for sc.Scan(ctx, d.scanSize) {
		contents := sc.Contents()
		numEntries += int64(len(contents))
		prune, err := d.handleContents(ctx, dirName, contents, numEntries)
		if err != nil {
			d.visit(dirName, "", filewalk.Entry{}, nil, err)
		}
		if prune {
			return prune, nil
		}

	}
	return false, sc.Err()
}

func (d *depthFirst) handleContents(ctx context.Context, parent string, contents []filewalk.Entry, numEntries int64) (bool, error) {
	if d.needsStat {
		return d.handleContentsWithStat(ctx, parent, contents, numEntries)
	}
	return d.handleContentsWithoutStat(ctx, parent, contents, numEntries)
}

func (d *depthFirst) handleContentsWithoutStat(ctx context.Context, parent string, contents []filewalk.Entry, numEntries int64) (bool, error) {
	dirs := make([]filewalk.Entry, 0, len(contents))
	for _, c := range contents {
		if c.IsDir() {
			dirs = append(dirs, c)
		}
	}
	// Stat the directories only.
	dirEntries, _, err := d.stats.Process(ctx, parent, dirs)
	if err != nil {
		// the only non-nil error will be a context cancellation.
		return true, err
	}

	dirMap := make(map[string]file.Info)
	for _, de := range dirEntries {
		dirMap[de.Name()] = de
	}
	for _, c := range contents {
		wn := entryType{
			name:       c.Name,
			path:       d.fs.Join(parent, c.Name),
			mode:       c.Type,
			numEntries: numEntries,
		}
		if d.expr.Eval(wn) {
			d.visit(parent, c.Name, c, nil, nil)
		}
		if !c.IsDir() {
			continue
		}
		de := dirMap[c.Name]
		prune, err := d.handleDir(ctx, wn.path, de)
		if err != nil {
			d.visit(d.fs.Join(parent, c.Name), "", filewalk.Entry{}, nil, err)
		}
		if prune {
			return prune, nil
		}
	}
	return false, nil
}

func (d *depthFirst) handleContentsWithStat(ctx context.Context, parent string, contents []filewalk.Entry, numEntries int64) (bool, error) {
	_, all, err := d.stats.Process(ctx, parent, contents)
	if err != nil {
		// the only non-nil error will be a context cancellation.
		return false, err
	}
	for i, c := range all {
		ws := withStat{
			ctx:        ctx,
			name:       c.Name(),
			path:       d.fs.Join(parent, c.Name()),
			fs:         d.fs,
			info:       c,
			numEntries: numEntries,
		}
		if d.expr.Eval(ws) {
			info := c
			d.visit(parent, c.Name(), contents[i], &info, nil)
		}
		if c.IsDir() {
			prune, err := d.handleDir(ctx, d.fs.Join(parent, c.Name()), c)
			if err != nil {
				d.visit(d.fs.Join(parent, c.Name()), "", filewalk.Entry{}, nil, err)
				continue
			}
			if prune {
				return prune, nil
			}
			continue
		}

	}
	return false, nil
}
