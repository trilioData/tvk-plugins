package shell

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

// CmdOut structure contains command output & exitcode
type CmdOut struct {
	Out      string
	ExitCode int
}

// RunCmd Execute given shell commands
// params:
// input=>cmd: formatted command string which needs to be executed.
// output=>*cmdOut: command output struct returned after command execution.
//			error: non-nil error if command execution failed.
func RunCmd(cmd string, env ...string) (*CmdOut, error) {
	outStruct, err := Execute(env, true, "%s", cmd)
	if err != nil {
		return outStruct, err
	}
	return outStruct, nil
}

func RunCmdWithOutput(command string) error {
	parts := strings.Split(command, " ")
	var argStrings []string
	if len(parts) > 0 {
		argStrings = parts[1:]
	}

	// suppress linter here issue -> G204: Subprocess launched with function call as argument or cmd arguments
	cmd := exec.Command(parts[0], argStrings...) //nolint:gosec // no other options here
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
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
