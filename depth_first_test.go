// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"cloudeng.io/cmdutil/flags"
	"cloudeng.io/file/filewalk/filewalktestutil"
)

type details struct {
	uid, gid string
	mode     string
}

func fillTree(ordered []string, parent string, out *strings.Builder, nEntries, left, level int) []string {
	if left == 0 {
		return ordered
	}
	indent := strings.Repeat("  ", level)
	for i := 0; i < nEntries; i++ {
		name := fmt.Sprintf("e-%v-%v", level, i)
		ordered = append(ordered, path.Join(parent, name))
		out.WriteString(indent)
		if i%2 == 1 {
			out.WriteString("- file:\n")
			out.WriteString(indent)
			out.WriteString(fmt.Sprintf("    name: %s\n", name))
			continue
		}
		out.WriteString("- dir:\n")
		out.WriteString(indent)
		out.WriteString(fmt.Sprintf("    name: %s\n", name))
		out.WriteString(indent)
		out.WriteString(fmt.Sprintf("    entries:\n"))
		ordered = fillTree(ordered, path.Join(parent, name), out, nEntries, left-1, level+2)
	}
	return ordered
}

func yamlForMockFS(name string, entries, depth int, ordered []string) (string, []string) {
	var out strings.Builder
	out.WriteString("name: ")
	out.WriteString(name)
	out.WriteString("\nentries:\n")
	ordered = fillTree(ordered, name, &out, entries, depth, 0)
	return out.String(), ordered
}

func TestDepthFirst(t *testing.T) {
	ctx := context.Background()

	spec, ordered := yamlForMockFS("root", 20, 2, []string{})
	fs, err := filewalktestutil.NewMockFS("root", filewalktestutil.WithYAMLConfig(spec))
	if err != nil {
		t.Fatal(err)
	}
	expected := []found{}
	for _, o := range ordered {
		expected = append(expected, found{path.Dir(o), path.Base(o), nil})
	}
	for _, sorted := range []bool{false, true} {
		for _, long := range []bool{false, true} {
			lf := &locateFlags{Sorted: sorted, Long: long}
			lf.ScanSize = 100
			collect := &collector{}
			lc := locateCmd{}
			if err := lc.locateFS(ctx, fs, lf, collect.visit, []string{"root"}); err != nil {
				t.Fatal(err)
			}
			analyzeDiffs(t, "sorted", collect.found, expected)
			if sorted {
				if !slices.Equal(collect.found, expected) {
					t.Errorf("got %v, want %v", collect.found, expected)
				}
			}
		}
	}
}

const withDeviceSpec = `
name: r
device: 30
file_id: 40
entries:
  - file:
	  name: f0
	  device: 30
	  file_id: 30
  - file:
	  name: f1
	  device: 30
	  file_id: 31
  - dir:
	  name: d0
	  device: 40
	  file_id: 40
	  entries:
		- file:
			name: f3
			device: 40
			file_id: 41
		- file:
			name: f4
			device: 40
			file_id: 42
`

func TestSameDevice(t *testing.T) {
	ctx := context.Background()
	fs, err := filewalktestutil.NewMockFS("r", filewalktestutil.WithYAMLConfig(withDeviceSpec))
	if err != nil {
		t.Fatal(err)
	}
	all := []string{"r/f0", "r/f1", "r/d0", "r/d0/f3", "r/d0/f4"}
	sameDevice := []string{"r/f0", "r/f1", "r/d0"}
	for _, tc := range []struct {
		sorted, sameDevice bool
		expected           []string
	}{
		{true, false, all},
		{false, false, all},
		{true, true, sameDevice},
		{false, true, sameDevice},
	} {
		lf := &locateFlags{Sorted: tc.sorted, SameDevice: tc.sameDevice}
		lf.ScanSize = 100
		collect := &collector{}
		lc := locateCmd{}
		if err := lc.locateFS(ctx, fs, lf, collect.visit, []string{"r"}); err != nil {
			t.Fatal(err)
		}
		paths := []string{}
		for _, found := range collect.found {
			paths = append(paths, path.Join(found.prefix, found.name))
		}
		if got, want := paths, tc.expected; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestExclusions(t *testing.T) {
	ctx := context.Background()

	spec, ordered := yamlForMockFS("root", 10, 4, []string{})
	fs, err := filewalktestutil.NewMockFS("root", filewalktestutil.WithYAMLConfig(spec))
	if err != nil {
		t.Fatal(err)
	}

	var exclusions flags.Repeating
	if err := exclusions.Set(".*-2-.*"); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(exclusions.String())

	excluded := 0
	expected := []found{}
	for _, o := range ordered {
		if re.MatchString(path.Dir(o)) {
			excluded++
			continue
		}
		expected = append(expected, found{path.Dir(o), path.Base(o), nil})
	}
	if excluded == 0 {
		t.Fatal("no exclusions")
	}
	for _, sorted := range []bool{false, true} {
		for _, long := range []bool{false, true} {
			lf := &locateFlags{Sorted: sorted, Long: long, Exclusions: exclusions}
			lf.ScanSize = 100
			collect := &collector{}
			lc := locateCmd{}
			if err := lc.locateFS(ctx, fs, lf, collect.visit, []string{"root"}); err != nil {
				t.Fatal(err)
			}
			analyzeDiffs(t, "exclusions", collect.found, expected)
		}
	}
}
