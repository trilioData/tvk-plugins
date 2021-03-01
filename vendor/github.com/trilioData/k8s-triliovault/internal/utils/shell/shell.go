package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CmdOut structure contains command output & exitcode
type CmdOut struct {
	Out      string
	ExitCode int
}

// Execute the given command.
func Execute(env []string, combinedOutput bool, format string, args ...interface{}) (*CmdOut, error) {
	s := fmt.Sprintf(format, args...)
	// TODO: escape handling
	parts := strings.Split(s, " ")

	var p []string
	for i := 0; i < len(parts); i++ {
		if parts[i] != "" {
			p = append(p, parts[i])
		}
	}

	var argStrings []string
	if len(p) > 0 {
		argStrings = p[1:]
	}
	return ExecuteArgs(env, combinedOutput, parts[0], argStrings...)
}

// ExecuteArgs execute given command
func ExecuteArgs(env []string, combinedOutput bool, name string, args ...string) (*CmdOut, error) {
	if log.IsLevelEnabled(log.DebugLevel) {
		cmd := strings.Join(args, " ")
		cmd = name + " " + cmd
		log.Debugf("Executing command: %s", cmd)
	}

	c := exec.Command(name, args...)
	c.Env = append(os.Environ(), env...)

	var b []byte
	var err error
	if combinedOutput {
		// Combine stderr and stdout in b.
		b, err = c.CombinedOutput()
	} else {
		// Just return stdout in b.
		b, err = c.Output()
	}

	if err != nil || !c.ProcessState.Success() {
		log.Debugf("Command[%s] => (FAILED) %s", name, string(b))
	} else {
		log.Debugf("Command[%s] => %s", name, string(b))
	}

	return &CmdOut{
		Out:      string(b),
		ExitCode: c.ProcessState.ExitCode(),
	}, err
}
