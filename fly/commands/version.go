package commands

import (
	"fmt"
	"github.com/concourse/concourse"
	"os"
)

type VersionCommand struct {}

func (command *VersionCommand) Execute([]string) error {
	printFlyVersion()
	return nil
}

func init() {
	Fly.VersionArg = printFlyVersion
}

func printFlyVersion() {
	fmt.Println(concourse.Version)
	os.Exit(0)
}
