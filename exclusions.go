// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"regexp"

	"cloudeng.io/file"
)

type exclusions struct {
	regexps []*regexp.Regexp
}

func newExclusions(patterns []string) (exclusions, error) {
	var ex exclusions
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return exclusions{}, err
		}
		ex.regexps = append(ex.regexps, re)
	}
	return ex, nil
}

func (e exclusions) Match(path string) bool {
	if len(e.regexps) == 0 {
		return false
	}
	for _, re := range e.regexps {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

type sameDevice struct {
	device uint64
}

func newSameDevice(ctx context.Context, fs file.FS, pathname string) (sameDevice, error) {
	info, err := fs.Stat(ctx, pathname)
	if err != nil {
		return sameDevice{}, err
	}
	xattr, err := fs.XAttr(ctx, pathname, info)
	if err != nil {
		return sameDevice{}, err
	}
	return sameDevice{device: xattr.Device}, nil
}

func (sd sameDevice) Match(ctx context.Context, fs file.FS, dirName string, dirInfo file.Info) (bool, error) {
	if sd.device == 0 {
		return true, nil
	}
	xattr, err := fs.XAttr(ctx, dirName, dirInfo)
	if err != nil {
		return false, err
	}
	return sd.device == xattr.Device, nil
}
