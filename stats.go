// Copyright 2023 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"sync"

	"cloudeng.io/algo/container/heap"
	"cloudeng.io/file/diskusage"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type StatsFlags struct {
	Stats bool `subcmd:"stats,false,display statistics on the files and directories"`
	Top   int  `subcmd:"top,50,display the top N files and directories by size"`
}

type Stats struct {
	enabled          bool
	mu               sync.Mutex
	topn             int
	fileSize         *heap.MinMax[int64, string]
	directorySize    *heap.MinMax[int64, string]
	totalSize        int64
	totalFiles       int64
	totalDirectories int64
}

var printer = message.NewPrinter(language.English)

func NewStats(fl *StatsFlags) *Stats {
	return &Stats{
		enabled:       fl.Stats,
		topn:          fl.Top,
		fileSize:      heap.NewMinMax[int64, string](),
		directorySize: heap.NewMinMax[int64, string](),
	}
}

func (s *Stats) UpdateFile(path string, size int64) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fileSize.PushMaxN(size, path, s.topn)
	s.totalSize += size
	s.totalFiles++
}

func (s *Stats) UpdateDir(path string, count int64) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.directorySize.PushMaxN(count, path, s.topn)
	s.totalDirectories++

}

func fmtSize(s int64) (bin, dec string) {
	v, u := diskusage.Base2Bytes(s).Standardize()
	bin = printer.Sprintf("%.2f%v", v, u)
	v, u = diskusage.DecimalBytes(s).Standardize()
	dec = printer.Sprintf("%.2f%v", v, u)
	return
}

func fmtCount(count int64) string {
	return printer.Sprintf("%v", count)
}

func (s *Stats) Write(out io.Writer) {
	if !s.enabled {
		return
	}
	fmt.Fprintf(out, "Total Files       : %v\n", fmtCount(s.totalFiles))
	fmt.Fprintf(out, "Total Directories : %v\n", fmtCount(s.totalDirectories))

	bin, dec := fmtSize(s.totalSize)
	fmt.Fprintf(out, "Total Size        : %v (%v)\n", bin, dec)

	fmt.Printf("Largest %v Files\n", s.topn)
	for s.fileSize.Len() > 0 {
		v, k := s.fileSize.PopMax()
		bin, dec := fmtSize(v)
		fmt.Printf("%v: %v (%v)\n", k, bin, dec)
	}

	fmt.Printf("Largest %v Directories\n", s.topn)
	for s.directorySize.Len() > 0 {
		v, k := s.directorySize.PopMax()
		fmt.Printf("%v: %v\n", k, fmtCount(v))
	}
}
