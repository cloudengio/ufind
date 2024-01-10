// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

/*

type ordinalSeen struct {
	prefixOrder  int64
	contentOrder int64
	name         string
}

type ordinalScanner struct {
	sync.Mutex
	fs   filewalk.FS
	seen []ordinalSeen
}

type ordinalState struct {
	prefixID, contentID int64
}

func (o *ordinalScanner) append(prefix, name string, prefixOrder, contentOrder int64) {
	o.Lock()
	defer o.Unlock()
	name = path.Join(prefix, name)
	o.seen = append(o.seen, ordinalSeen{
		name:         name,
		prefixOrder:  prefixOrder,
		contentOrder: contentOrder,
	})
}

func (o *ordinalScanner) Prefix(_ context.Context, ordinal int64, state *ordinalState, prefix string, _ file.Info, _ error) (bool, file.InfoList, error) {
	//	o.append(prefix, "", ordinal, 0)
	state.prefixID = ordinal
	return false, nil, nil
}

func (o *ordinalScanner) Contents(ctx context.Context, state *ordinalState, prefix string, contents []filewalk.Entry) (file.InfoList, error) {
	var children file.InfoList
	state.contentID++
	for _, de := range contents {
		o.append(prefix, de.Name, state.prefixID, state.contentID)
		if !de.IsDir() {
			continue
		}
		info, err := o.fs.Lstat(ctx, o.fs.Join(prefix, de.Name))
		if err != nil {
			return nil, err
		}
		children = append(children, info)
	}
	return children, nil
}

func (o *ordinalScanner) Done(_ context.Context, _ *ordinalState, _ string, _ error) error {
	return nil
}

func writeFiles(parent string, out *strings.Builder, nFiles int, level int, indent string) []string {
	files := []string{}
	for i := 0; i < nFiles; i++ {
		out.WriteString(indent)
		out.WriteString("- file:\n")
		out.WriteString(indent)
		name := fmt.Sprintf("f-%v-%v", level, i)
		out.WriteString(fmt.Sprintf("    name: %s\n", name))
		files = append(files, path.Join(parent, name))
	}
	return files
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

func TestOrdinal(t *testing.T) {
	defer synctestutil.AssertNoGoroutines(t)()

	ctx := context.Background()
	spec, ordered := yamlForMockFS("root", 20, 2, []string{})
	fs, err := filewalktestutil.NewMockFS("root", filewalktestutil.WithYAMLConfig(spec))
	if err != nil {
		t.Fatal(err)
	}
	o := &ordinalScanner{fs: fs}

	wk := filewalk.New(fs, o, filewalk.WithScanSize(2), filewalk.WithConcurrentScans(100))
	if err := wk.Walk(ctx, "root"); err != nil {
		t.Fatal(err)
	}

	//	for i, n := range ordered {
	//		fmt.Printf("%v: %v...\n", i, n)
	//	}

	if got, want := len(o.seen), len(ordered); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	names := []string{}
	for _, s := range o.seen {
		names = append(names, s.name)
	}

	if slices.Equal(names, ordered) {
		t.Errorf("names should not be presorted..")
	}

	sorted := slices.Clone(o.seen)

	sort.Slice(sorted, func(i, j int) bool {
		id, jd := strings.Count(sorted[i].name, "/"), strings.Count(sorted[j].name, "/")
		if id != jd {
			return id < jd
		}
		if sorted[i].prefixOrder != sorted[j].prefixOrder {
			return sorted[i].prefixOrder < sorted[j].prefixOrder
		}
		return sorted[i].contentOrder < sorted[j].contentOrder
	})

	names = []string{}
	for _, s := range sorted {
		names = append(names, s.name)
	}

	fmt.Printf("names: %v\n", strings.Join(names, "\n"))
	fmt.Printf("ordered: %v\n", strings.Join(ordered, "\n"))

	if !slices.Equal(names, ordered) {
		t.Errorf("names should be sorted..")
	}

	t.Fail()
}
*/
