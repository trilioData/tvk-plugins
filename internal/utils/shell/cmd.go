package shell

import (
	"fmt"
	"io/ioutil"
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

// RunCmd Execute given shell commands
// params:
// input=>cmd: formatted command string which needs to be executed.
// output=>*cmdOut: command output struct returned after command execution.
//
//	error: non-nil error if command execution failed.
func RunCmd(cmd string, env ...string) (*CmdOut, error) {
	outStruct, err := Execute(env, true, "%s", cmd)
	if err != nil {
		return outStruct, err
	}
	return outStruct, nil
}

// ChmodR changes permission of a file
// params:
// input=> path: filepath to change file permission
//
//	mode: permissions
//
// output=> outStruct.out:stdout string
//
//	err: non-nil error if command execution failed.
func ChmodR(dirPath, mode string) (string, error) {
	// Recursive chmod
	cmd := fmt.Sprintf("chmod -R %s %s", mode, dirPath)
	outStruct, err := RunCmd(cmd)
	if err != nil {
		log.Errorf("Failed: [%s]", outStruct.Out)
		return outStruct.Out, err
	}
	return outStruct.Out, nil
}

// RmRf removes specified file
// param:
// input=> path: filePath to remove
// output=> err.Error(): returns error string if command execution fails.
//
//	outStr: returns stdout
//	err: non-nil error if command execution failed.
func RmRf(dirPath string) (string, error) {
	// Recursively removes files/directory
	fi, err := os.Lstat(dirPath)
	if err != nil {
		return err.Error(), err
	}

	outStr, err := ChmodR(dirPath, "0775")
	if err != nil {
		log.Errorf("Failed: [%s]", outStr)
		return outStr, err
	}

	switch mode := fi.Mode(); {
	case mode.IsRegular():
		err = os.Remove(dirPath)
		if err != nil {
			return err.Error(), err
		}
	case mode.IsDir():
		err = os.RemoveAll(dirPath)
		if err != nil {
			return err.Error(), err
		}
	}
	return "", nil
}

func ReadChildDir(dirPath string) (dirNames []string, err error) {
	// ReadChildDir reads the dir name from the given path
	// Input:
	//		dirPath: Directory path
	// Output:
	//		dirNames: Directory name list
	//		err: Error

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return dirNames, err
	}
	for _, file := range files {
		if file.IsDir() {
			dirNames = append(dirNames, file.Name())
		}
	}

	return dirNames, nil
}

// Mkdir creates directory with specified name
// params:
// input=> directory: directory path to create.
// output=> err.Error(): returns error string if command execution fails
//
//	err: non-nil error if command execution failed.
func Mkdir(directory string) (string, error) {
	_, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			mkdirCmd := fmt.Sprintf("sudo mkdir %s", directory)
			cmd := exec.Command("bash", "-c", mkdirCmd)
			_, err = cmd.CombinedOutput()
			if err != nil {
				return err.Error(), err
			}
			return "", nil

		}
		return err.Error(), err
	}
	return "", nil
}
