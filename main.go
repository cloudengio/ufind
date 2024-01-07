package main

import (
	"context"

	"cloudeng.io/cmdutil/subcmd"
)

const commands = `name: ufind
summary: ultra fast, parallel, find command

commands:
  - name: locate
    summary: locate files using boolean expressions
    arguments:
      - <directory>
      - <expression>...
  - name: expression-syntax
    summary: show help on the expression syntax and matching operations
 `

func cli() *subcmd.CommandSetYAML {
	cmdSet := subcmd.MustFromYAMLTemplate(commands)
	locate := locateCmd{}
	cmdSet.Set("locate").MustRunner(locate.locate, &locateFlags{})
	cmdSet.Set("expression-syntax").MustRunner(locate.explain, &struct{}{})
	return cmdSet
}

func main() {
	cli().MustDispatch(context.Background())
}
