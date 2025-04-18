/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

var bashCompletionTemplate = `_crictl() {
    local cur opts base
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    opts="%s"
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _crictl crictl`

func bashCompletion(c *cli.Context) error {
	subcommands := []string{}

	for _, command := range c.App.Commands {
		if command.Hidden {
			continue
		}

		subcommands = append(subcommands, command.Names()...)
	}

	for _, flag := range c.App.Flags {
		// only includes full flag name.
		subcommands = append(subcommands, "--"+flag.Names()[0])
	}

	fmt.Fprintln(c.App.Writer, fmt.Sprintf(bashCompletionTemplate, strings.Join(subcommands, "\n")))

	return nil
}

var zshCompletionTemplate = `#compdef crictl

_crictl() {
  local -a cmds
  cmds=('%s')
  _describe 'commands' cmds

  local -a opts
  opts=('%s')
  _describe 'global options' opts

  return
}

compdef _crictl crictl`

func zshCompletion(c *cli.Context) error {
	subcommands := []string{}

	for _, command := range c.App.Commands {
		if command.Hidden {
			continue
		}

		for _, name := range command.Names() {
			subcommands = append(subcommands, name+":"+command.Usage)
		}
	}

	opts := []string{}
	for _, flag := range c.App.Flags {
		// only includes full flag name.
		opts = append(opts, "--"+flag.Names()[0])
	}

	fmt.Fprintln(c.App.Writer, fmt.Sprintf(zshCompletionTemplate, strings.Join(subcommands, "' '"), strings.Join(opts, "' '")))

	return nil
}

func fishCompletion(c *cli.Context) error {
	completion, err := c.App.ToFishCompletion()
	if err != nil {
		return err
	}

	fmt.Fprintln(c.App.Writer, completion)

	return nil
}

var completionCommand = &cli.Command{
	Name:      "completion",
	Usage:     "Output shell completion code",
	ArgsUsage: "SHELL",
	Description: `Output shell completion code for bash, zsh or fish.

Examples:

    # Installing bash completion on Linux
    source <(crictl completion bash)

    # Installing zsh completion on Linux
    source <(crictl completion zsh)

    # Installing fish completion on Linux
    crictl completion fish | source
	`,
	Action: func(c *cli.Context) error {
		// select bash by default for backwards compatibility
		if c.NArg() == 0 {
			return bashCompletion(c)
		}

		if c.NArg() != 1 {
			return cli.ShowSubcommandHelp(c)
		}

		switch c.Args().First() {
		case "bash":
			return bashCompletion(c)
		case "fish":
			return fishCompletion(c)
		case "zsh":
			return zshCompletion(c)
		default:
			return errors.New("only bash, zsh or fish are supported")
		}
	},
}
