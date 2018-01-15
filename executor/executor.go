package executor

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/screwdriver-cd/sd-cmd/config"
	"github.com/screwdriver-cd/sd-cmd/logger"
	"github.com/screwdriver-cd/sd-cmd/screwdriver/api"
	"github.com/screwdriver-cd/sd-cmd/util"
)

var lagger *logger.Logger

// Executor is a Executor endpoint
type Executor interface {
	Run() error
}

func prepareLog(namespace, name, version string) (err error) {
	dirPath := filepath.Join(config.SDArtifactsDir, ".sd", "commands", namespace, name, version)
	filename := fmt.Sprintf("%v.log", time.Now().Unix())
	lagger, err = logger.New(dirPath, filename, log.LstdFlags, false)
	if err != nil {
		return err
	}
	return nil
}

// New returns each format type of Executor
func New(sdAPI api.API, args []string) (Executor, error) {
	ns, name, ver, itr, err := util.SplitCmdWithSearch(args)
	if err != nil {
		return nil, err
	}

	err = prepareLog(ns, name, ver)
	if err != nil {
		return nil, err
	}

	spec, err := sdAPI.GetCommand(ns, name, ver)
	if err != nil {
		return nil, err
	}
	switch spec.Format {
	case "binary":
		return NewBinary(spec, args[itr+1:])
	case "habitat":
		return nil, nil
	case "docker":
		return nil, nil
	}
	return nil, nil
}

func writeCommandLog(count int, content chan string, finish chan bool, done chan bool) {
	laggerCommand := new(logger.Logger)
	laggerCommand.SetInfos(lagger.File, 0, true)
	for {
		select {
		case c := <-content:
			laggerCommand.Debug.Printf("%v", c)
		case fin := <-finish:
			if fin {
				count--
			}
		}
		if count <= 0 {
			break
		}
	}
	done <- true
}

func execCommand(path string, args []string) error {
	cmd := exec.Command(path, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe for exec command: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe for exec command: %v", err)
	}

	content := make(chan string)
	finish := make(chan bool)
	done := make(chan bool)
	lagger.Debug.Println("mmmmmm START COMMAND OUTPUT mmmmmm")
	go func(content chan string, finish chan bool) {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			content <- fmt.Sprintf("%v\n", scanner.Text())
		}
		finish <- true
	}(content, finish)

	go func(content chan string, finish chan bool) {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			content <- fmt.Sprintf("%v\n", scanner.Text())
		}
		finish <- true
	}(content, finish)

	go writeCommandLog(2, content, finish, done)

	err = cmd.Run()

	<-done
	close(content)
	close(finish)
	close(done)
	lagger.Debug.Println("mmmmmm FINISH COMMAND OUTPUT mmmmmm")
	state := cmd.ProcessState
	lagger.Debug.Printf("System Time: %v, User Time: %v\n", state.SystemTime(), state.UserTime())
	if err != nil {
		return fmt.Errorf("failed to exec command: %v", err)
	}
	return nil
}
