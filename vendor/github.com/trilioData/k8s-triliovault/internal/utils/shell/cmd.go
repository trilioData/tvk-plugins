package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

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

// ChmodR changes permission of a file
// params:
// input=> path: filepath to change file permission
// 		   mode: permissions
// output=> outStruct.out:stdout string
// 			err: non-nil error if command execution failed.
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
// 			outStr: returns stdout
// 			err: non-nil error if command execution failed.
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

// Mkdir creates directory with specified name
// params:
// input=> directory: directory path to create.
// output=> err.Error(): returns error string if command execution fails
// 			err: non-nil error if command execution failed.
func Mkdir(directory string) (string, error) {
	_, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(directory, 0777)
			if err != nil {
				return err.Error(), err
			}
			return "", nil
		}
		return err.Error(), err
	}
	return "", nil
}

// CreateFile creates a file with specified path
// params:
// input=> filePath: filePath to create file.
// output=> err.Error(): returns error string if command execution fails
// 			err: non-nil error if command execution failed.
func CreateFile(filePath string) (string, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, createErr := os.Create(filePath)
			if createErr != nil {
				log.Errorf("failed:[%s]", createErr.Error())
				return createErr.Error(), createErr
			}
			log.Debugf("File created : [%s]", file.Name())
			return "", nil
		}
		return err.Error(), err
	}
	return "", nil
}

// WriteToFile writes string to file with create if file doesn't exits
func WriteToFile(filePath, text string) error {

	_, err := CreateFile(filePath)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(text)
	if err != nil {
		return err
	}
	return file.Sync()
}

// FileExistsInDir check file  exists or not is directory
// params:
// input=> directory: directory name to check file in.
// 		   filename: filename to check if exists?
// output=> isExists:true if exists & false is not.
// 			outStr: contains err.Error() if command execution fails
// 					else return stdout.
func FileExistsInDir(directory, filename string) (isExists bool, outStr string, err error) {
	filePath := path.Join(directory, filename)
	_, err = os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		outStr, err = err.Error(), nil
		return isExists, outStr, err
	} else if err != nil {
		outStr = err.Error()
		return isExists, outStr, err
	}
	isExists = true
	return isExists, outStr, err
}

// Rename changes name of a file.
// params:
// input=> srcPath: source file/directory path to rename.
// 		   destPath: new file/directory path name
// output=> err.Error(): returns error string if command execution fails.
// 			outStr: return stdout.
// 			err: non-nil error if command execution failed.
func Rename(srcPath, destPath string) (string, error) {
	outStr, err := ChmodR(srcPath, "0775")
	if err != nil {
		log.Errorf("Failed:[%s]", outStr)
		return outStr, err
	}
	err = os.Rename(srcPath, destPath)
	if err != nil {
		return err.Error(), err
	}
	return "", nil
}

func RmRfLastDir(dirPath string) (string, error) {
	// Recursively removes files/directory from the last dir path including that dir
	log.Debugf("Removing the complete last directory from the path: %s", dirPath)
	dir := filepath.Dir(dirPath)
	base := filepath.Base(dirPath)

	files, err := filepath.Glob(filepath.Join(dir, base))
	matchingDir := files[0]
	if err != nil {
		log.Errorf("Error while fetching the contents of a path: %s, with matching dir: %s, error: %s", dirPath, matchingDir, err.Error())
		return err.Error(), err
	}

	err = os.RemoveAll(matchingDir)
	if err != nil {
		log.Errorf("Error while removing the dir: %s, error: %s", matchingDir, err.Error())
		return err.Error(), err
	}
	return "", nil
}

// ClearDir removes all the files and dirs from the passed dir
func ClearDir(dirPath string) (string, error) {
	log.Debugf("Removing all the files from the path: %s", dirPath)

	files, err := filepath.Glob(filepath.Join(dirPath, "*"))
	if err != nil {
		log.Errorf("Error while fetching the contents of a path: %s, error: %s", dirPath, err.Error())
		return err.Error(), err
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			log.Errorf("Error while removing the contents of a path: %s, error: %s", file, err.Error())
			return err.Error(), err
		}
	}
	return "", nil
}

// CopyDir copies complete directory recursively from src to dst
func CopyDir(src, dst string) error {
	cmd := fmt.Sprintf("cp -r %s %s", src, dst)
	return RunCmdWithOutput(cmd)
}
