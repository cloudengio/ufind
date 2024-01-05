// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"cloudeng.io/cmd/ufind/internal"
	"cloudeng.io/file"
	"cloudeng.io/file/filewalk"
	"cloudeng.io/file/filewalk/localfs"
)

var localTestTree string

func TestMain(m *testing.M) {
	localTestTree = internal.CreateTestTree()
	if code := m.Run(); code != 0 {
		fmt.Printf("tmpdir: %v\n", localTestTree)
		os.Exit(code)
	}
	os.RemoveAll(localTestTree)
	os.Exit(0)
}

func newExpr(t *testing.T, prune bool, expr string) expression {
	t.Helper()
	e, err := createExpr(prune, []string{expr})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestNeedsStat(t *testing.T) {
	newExpr := func(t *testing.T, expr string) expression {
		return newExpr(t, false, expr)
	}
	e := newExpr(t, "re=.go")

	if got, want := e.NeedsStat(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	e = newExpr(t, "re=.go || type=f")
	if got, want := e.NeedsStat(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	e = newExpr(t, "re=.go || newer=2010-12-13")
	if got, want := e.NeedsStat(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	e = newExpr(t, "type=f")
	if got, want := e.NeedsStat(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	e = newExpr(t, "type=x")
	if got, want := e.NeedsStat(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	e = newExpr(t, "file-larger=10")
	if got, want := e.NeedsStat(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := e.NeedsNumEntries(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	e = newExpr(t, "dir-larger=100")
	if got, want := e.NeedsNumEntries(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := e.NeedsStat(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

type found struct {
	prefix, name string
	err          error
}
type collector struct {
	sync.Mutex
	found []found
	errs  []found
}

func (c *collector) visit(prefix, name string, e filewalk.Entry, fi *file.Info, err error) {
	c.Lock()
	defer c.Unlock()
	prefix = strings.TrimPrefix(prefix, localTestTree)
	if err != nil {
		c.errs = append(c.errs, found{prefix, name, err})
		return
	}
	c.found = append(c.found, found{prefix, name, nil})
}

func sortFound(f []found) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].prefix == f[j].prefix {
			return f[i].name < f[j].name
		}
		return f[i].prefix < f[j].prefix
	})
}

func locate(ctx context.Context, t *testing.T, lf *locateFlags, args ...string) ([]found, []found) {
	fs := localfs.New()

	collect := &collector{}
	lc := locateCmd{}
	if err := lc.locateFS(ctx, fs, lf, collect.visit, args); err != nil {
		t.Fatal(err)
	}
	sortFound(collect.found)
	sortFound(collect.errs)
	return collect.found, collect.errs
}

func printFound(t *testing.T, found []found) {
	for _, f := range found {
		t.Logf("p: %v, n: %v\n", strings.TrimPrefix(f.prefix, localTestTree), f.name)
	}
}

func asMap(got []found) map[string]found {
	m := map[string]found{}
	for _, f := range got {
		p := strings.TrimPrefix(f.prefix, localTestTree)
		m[fmt.Sprintf("%v??%v", p, f.name)] = f
	}
	return m
}

func analyzeDiffs(t *testing.T, m string, got, want []found) {
	gm := asMap(got)
	wm := asMap(want)
	fmt.Printf("GM: %v\n", gm)
	fmt.Printf("WM: %v\n", wm)
	for k := range gm {
		if _, ok := wm[k]; !ok {
			t.Logf("%v: got: %v, not in want (%v)\n", m, gm[k], k)
		}
	}
	for k := range wm {
		if _, ok := gm[k]; !ok {
			t.Logf("%v: want: %v, not in got (%v)\n", m, wm[k], k)
		}
	}
	t.Logf("---- got  ----\n")
	printFound(t, got)
	t.Logf("---- want ----\n")
	printFound(t, want)
}

func cmpFound(t *testing.T, found []found, expected []found) {
	_, _, line, _ := runtime.Caller(1)
	if got, want := len(found), len(expected); got != want {
		analyzeDiffs(t, "mismatched len", found, expected)
		t.Fatalf("line %v, got %v, want %v", line, got, want)
	}

	for i := range found {
		if got, want := found[i].prefix, expected[i].prefix; got != want {
			analyzeDiffs(t, "wrong prefix", found, expected)
			t.Fatalf("line %v, got %v, want %v", line, got, want)
		}
		if got, want := found[i].name, expected[i].name; got != want {
			analyzeDiffs(t, "wrong name", found, expected)
			t.Fatalf("line %v, got %v, want %v", line, got, want)
		}
	}
}

func zipf(a []string, b ...string) []found {
	z := make([]found, 0, len(a))
	for i := range a {
		z = append(z, found{
			prefix: strings.ReplaceAll(a[i], "/", string(filepath.Separator)),
			name:   strings.ReplaceAll(b[i], "/", string(filepath.Separator))})
	}
	return z
}

func zips(a ...string) []string {
	return a
}

func all() []found {
	entries := []string{
		"/f2",
		"/inaccessible-dir",
		"/a0",
		"/a0/f2",
		"/a0/inaccessible-file",
		"/a0/a0.0",
		"/a0/a0.0/f2",
		"/a0/a0.0/f0",
		"/a0/a0.0/f1",
		"/a0/inaccessible-dir",
		"/a0/f0",
		"/a0/f1",
		"/a0/a0.1",
		"/a0/a0.1/f2",
		"/a0/a0.1/f0",
		"/a0/a0.1/f1",
		"/lf0",
		"/b0",
		"/b0/b0.0",
		"/b0/b0.0/f2",
		"/b0/b0.0/f0",
		"/b0/b0.0/f1",
		"/b0/b0.1",
		"/b0/b0.1/b1.0",
		"/b0/b0.1/b1.0/f2",
		"/b0/b0.1/b1.0/f0",
		"/b0/b0.1/b1.0/f1",
		"/f0",
		"/la0",
		"/f1",
		"/la1",
	}
	f := []found{}
	for _, dir := range entries {
		d := filepath.Dir(dir)
		if d == "/" {
			d = ""
		}
		f = append(f, found{d, filepath.Base(dir), nil})
	}
	sortFound(f)
	return f
}

var allFiles = zipf(zips(
	"", "", "",
	"/a0", "/a0", "/a0", "/a0",
	"/a0/a0.0", "/a0/a0.0", "/a0/a0.0",
	"/a0/a0.1", "/a0/a0.1", "/a0/a0.1",
	"/b0/b0.0", "/b0/b0.0", "/b0/b0.0",
	"/b0/b0.1/b1.0", "/b0/b0.1/b1.0", "/b0/b0.1/b1.0"),
	"f0", "f1", "f2",
	"f0", "f1", "f2", "inaccessible-file",
	"f0", "f1", "f2",
	"f0", "f1", "f2",
	"f0", "f1", "f2",
	"f0", "f1", "f2")

var allDirs = zipf(zips(
	"", "", "",
	"/a0", "/a0", "/a0",
	"/b0", "/b0",
	"/b0/b0.1"),
	"a0", "b0", "inaccessible-dir",
	"a0.0", "a0.1", "inaccessible-dir",
	"b0.0", "b0.1",
	"b1.0")

func init() {
	sortFound(allFiles)
	sortFound(allDirs)
}

func TestNamesAndPaths(t *testing.T) {
	ctx := context.Background()

	expectedErrors := zipf(zips("/a0/inaccessible-dir", "/inaccessible-dir"), "", "")
	found, foundErrors := locate(ctx, t, &locateFlags{}, localTestTree, "")
	cmpFound(t, found, all())
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "re=a0$ || re=b0.1$")
	cmpFound(t, found, zipf(zips("", "", "/b0"), "a0", "la0", "b0.1"))
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "re=a0$ || re=b0.1$ || type=x")
	cmpFound(t, found, zipf(zips("", "", "/b0"), "a0", "la0", "b0.1"))
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "re=a0/a0.1")
	cmpFound(t, found, zipf(zips("/a0", "/a0/a0.1", "/a0/a0.1", "/a0/a0.1"), "a0.1", "f0", "f1", "f2"))
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "type=f")
	cmpFound(t, found, allFiles)
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "type=d")
	cmpFound(t, found, allDirs)
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{Prune: true}, localTestTree, "type=l")
	cmpFound(t, found, zipf(zips("", "", ""), "la0", "la1", "lf0"))
	cmpFound(t, foundErrors, expectedErrors)

	// test prune
	found, foundErrors = locate(ctx, t, &locateFlags{Prune: true}, localTestTree, "")
	cmpFound(t, found, all())
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{Prune: true}, localTestTree, "name=a0")
	cmpFound(t, found, zipf(zips(""), "a0"))
	cmpFound(t, foundErrors, zipf(zips("/inaccessible-dir"), ""))

	found, foundErrors = locate(ctx, t, &locateFlags{Prune: true}, localTestTree, "name=a0.1")
	cmpFound(t, found, zipf(zips("/a0"), "a0.1"))
	cmpFound(t, foundErrors, expectedErrors)

}

func TestWithStats(t *testing.T) {
	ctx := context.Background()
	expectedErrors := zipf(zips("/a0/inaccessible-dir", "/inaccessible-dir"), "", "")

	found, foundErrors := locate(ctx, t, &locateFlags{}, localTestTree, "newer=2010-12-13")
	cmpFound(t, found, all())
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "newer=2050-12-13")
	cmpFound(t, found, nil)
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "file-larger=3")
	cmpFound(t, found, allFiles)
	cmpFound(t, foundErrors, expectedErrors)

	found, foundErrors = locate(ctx, t, &locateFlags{}, localTestTree, "file-larger=4")
	cmpFound(t, found, nil)
	cmpFound(t, foundErrors, expectedErrors)
}

func TestNumEntries(t *testing.T) {
	ctx := context.Background()
	found, _ := locate(ctx, t, &locateFlags{}, localTestTree, "dir-larger=1")
	cmpFound(t, found, allDirs)
	// Prune has no effect on dir-larger/dir-smaller.
	found, _ = locate(ctx, t, &locateFlags{Prune: true}, localTestTree, "dir-larger=1")
	cmpFound(t, found, allDirs)
	found, _ = locate(ctx, t, &locateFlags{}, localTestTree, "dir-larger=100")
	cmpFound(t, found, nil)
}
