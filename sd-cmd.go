package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"syscall"

	"github.com/screwdriver-cd/sd-cmd/config"
	"github.com/screwdriver-cd/sd-cmd/executor"
	"github.com/screwdriver-cd/sd-cmd/logger"
	"github.com/screwdriver-cd/sd-cmd/publisher"
	"github.com/screwdriver-cd/sd-cmd/screwdriver/api"
	"github.com/screwdriver-cd/sd-cmd/validator"
)

const (
	minArgLength           = 2
	defaultFailureExitCode = 1
)

func cleanExit() {
	logger.CloseAll()
}

func successExit() {
	cleanExit()
	os.Exit(0)
}

// failureExit exits process with 1
func failureExit(err error) {
	cleanExit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
	}
	os.Exit(defaultFailureExitCode)
}

// finalRecover makes one last attempt to recover from a panic.
// This should only happen if the previous recovery caused a panic.
func finalRecover() {
	if p := recover(); p != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Something terrible has happened. Please file a ticket with this info:")
		fmt.Fprintf(os.Stderr, "ERROR: %v\n%v\n", p, string(debug.Stack()))
		failureExit(nil)
	}
	successExit()
}

func init() {
	config.LoadConfig()
}

func runExecutor(sdAPI api.API, args []string) (err error) {
	exec, err := executor.New(sdAPI, args)
	if err != nil {
		return
	}
	err = exec.Run()
	return
}

func runPublisher(inputCommand []string) error {
	sdAPI := api.New(config.SDAPIURL, config.SDToken)
	pub, err := publisher.New(sdAPI, inputCommand)
	if err != nil {
		return fmt.Errorf("Fail to get publisher: %v", err)
	}
	return pub.Run()
}

func runValidator(inputCommand []string) error {
	sdAPI := api.New(config.SDAPIURL, config.SDToken)
	val, err := validator.New(sdAPI, inputCommand)
	if err != nil {
		return fmt.Errorf("Fail to get validator: %v", err)
	}
	return val.Run()
}

func runCommand(sdAPI api.API, args []string) error {
	if len(os.Args) < minArgLength {
		return fmt.Errorf("The number of arguments is not enough")
	}

	switch args[1] {
	case "exec":
		return runExecutor(sdAPI, args)
	case "publish":
		return runPublisher(args[2:])
	case "promote":
		return fmt.Errorf("promote is not implemented yet")
	case "validate":
		return runValidator(args[2:])
	default:
		return runExecutor(sdAPI, args)
	}
}

func main() {
	defer finalRecover()

	sdAPI := api.New(config.SDAPIURL, config.SDToken)

	err := runCommand(sdAPI, os.Args)
	if err != nil {
		failureExit(err)
	}
}
