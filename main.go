// Copyright 2024 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"cloudeng.io/aws/awsconfig"
	"cloudeng.io/cmdutil"
	"cloudeng.io/cmdutil/profiling"
	"cloudeng.io/cmdutil/subcmd"

	"cloudeng.io/cmd/ufind/internal/dynamic"
)

const commands = `name: ufind
summary: ultra fast, parallel, find command

commands:
  - name: locate
    summary: locate files using boolean expressions
    arguments:
      - <directory> - the directory to start the search from
      - <expression>... - the expression to match files against
  - name: expression-syntax
    summary: show help on the expression syntax and matching operations
 `

type GlobalFlags struct {
	ExitProfile profiling.ProfileFlag `subcmd:"profile,,'write a profile on exit; the format is <profile-name>:<file> and the flag may be repeated to request multiple profile types, use cpu to request cpu profiling in addition to predefined profiles in runtime/pprof'"`

	awsconfig.AWSFlags
}

var (
	globalFlags GlobalFlags
	metaFS      = dynamic.NewFS()
)

func cli() *subcmd.CommandSetYAML {
	cmdSet := subcmd.MustFromYAMLTemplate(commands)
	locate := locateCmd{}

	cmdSet.Set("locate").MustRunner(locate.locate, &locateFlags{})
	cmdSet.Set("expression-syntax").MustRunner(locate.explain, &struct{}{})

	globals := subcmd.GlobalFlagSet()
	globals.MustRegisterFlagStruct(&globalFlags, nil, nil)
	cmdSet.WithGlobalFlags(globals)
	cmdSet.WithMain(mainWrapper)
	return cmdSet
}

func mainWrapper(ctx context.Context, cmdRunner func(ctx context.Context) error) error {
	for _, profile := range globalFlags.ExitProfile.Profiles {
		if !profiling.IsPredefined(profile.Name) {
			fmt.Printf("warning profile %v defaults to CPU profiling since it is not one of the predefined profile types: %v", profile.Name, strings.Join(profiling.PredefinedProfiles(), ", "))
		}
		save, err := profiling.Start(profile.Name, profile.Filename)
		if err != nil {
			return err
		}
		fmt.Printf("profiling: %v %v\n", profile.Name, profile.Filename)
		defer save()
	}

	metaFS.Register("s3", dynamic.NewS3FS(globalFlags.AWSFlags))

	ctx, cancel := context.WithCancel(ctx)
	cmdutil.HandleSignals(cancel, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return cmdRunner(ctx)
}

func main() {
	cli().MustDispatch(context.Background())
}
