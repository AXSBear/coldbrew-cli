package main

import (
	"fmt"
	"os"

	"github.com/coldbrewcloud/coldbrew-cli/commands"
	"github.com/coldbrewcloud/coldbrew-cli/commands/clustercreate"
	"github.com/coldbrewcloud/coldbrew-cli/commands/clusterdelete"
	"github.com/coldbrewcloud/coldbrew-cli/commands/clusterstatus"
	"github.com/coldbrewcloud/coldbrew-cli/commands/create"
	"github.com/coldbrewcloud/coldbrew-cli/commands/delete"
	"github.com/coldbrewcloud/coldbrew-cli/commands/deploy"
	"github.com/coldbrewcloud/coldbrew-cli/console"
	"github.com/coldbrewcloud/coldbrew-cli/flags"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	appName = "coldbrew"
	appHelp = "(application description goes here)"
)

type CLIApp struct {
	kingpinApp *kingpin.Application
	appFlags   *flags.GlobalFlags
	commands   map[string]commands.Command
}

func main() {
	kingpinApp := kingpin.New(appName, appHelp)
	kingpinApp.Version(Version)
	globalFlags := flags.NewGlobalFlags(kingpinApp)

	// register commands
	registeredCommands := registerCommands(kingpinApp, globalFlags)

	// parse CLI inputs
	command, err := kingpinApp.Parse(os.Args[1:])
	if err != nil {
		console.Errorf(err.Error())
		os.Exit(5)
	}

	// setup debug logging
	console.EnableDebugf(*globalFlags.Verbose, "")

	// execute command
	if c := registeredCommands[command]; c != nil {
		if err := c.Run(); err != nil {
			console.Errorf("Error: %s\n", err.Error())
			os.Exit(40)
		}
		os.Exit(0)
	} else {
		panic(fmt.Errorf("Unknown command: %s", command))
	}
}

func registerCommands(ka *kingpin.Application, globalFlags *flags.GlobalFlags) map[string]commands.Command {
	registeredCommands := make(map[string]commands.Command)

	cmds := []commands.Command{
		&create.Command{},
		&deploy.Command{},
		&delete.Command{},
		&clustercreate.Command{},
		&clusterstatus.Command{},
		&clusterdelete.Command{},
	}
	for _, c := range cmds {
		kpc := c.Init(ka, globalFlags)
		registeredCommands[kpc.FullCommand()] = c
	}

	return registeredCommands
}
