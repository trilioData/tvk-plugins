package testutils

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
)

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(internal.InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}

// UpdateYAMLs Update old YAML values with new values
// fileOrDirPath can be a single file path or a directory
// kv is map of old value to new value
func UpdateYAMLs(kv map[string]string, fileOrDirPath string) error {
	var files []string
	info, err := os.Stat(fileOrDirPath)

	if os.IsNotExist(err) {
		return err
	}

	var walkFn = func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, path)
		}

		return nil
	}

	if info.IsDir() {
		if err := filepath.Walk(fileOrDirPath, walkFn); err != nil {
			return err
		}
	} else {
		files = append(files, fileOrDirPath)
	}

	for _, yamlPath := range files {
		read, readErr := os.ReadFile(yamlPath)
		if readErr != nil {
			return readErr
		}

		updatedFile := string(read)

		for placeholder, value := range kv {
			if strings.Contains(updatedFile, placeholder) {
				updatedFile = strings.ReplaceAll(updatedFile, placeholder, value)
				log.Infof("Updated the old value: [%s] with new value: [%s] in file [%s]",
					placeholder, value, yamlPath)
			}
		}

		if writeErr := os.WriteFile(yamlPath, []byte(updatedFile), 0); writeErr != nil {
			return writeErr
		}
	}
	return nil
}
