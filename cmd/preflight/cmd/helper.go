package cmd

import (
	"io"
	"os"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/reporters/stenographer/support/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/internal"
)

func setupLogger(logFilePrefix string) error {
	var err error
	preflightLogFilename = generateLogFileName(logFilePrefix)
	logFile, err = os.OpenFile(preflightLogFilename, os.O_CREATE|os.O_WRONLY, filePermission)
	if err != nil {
		log.Errorf("Unable to create log file - %s. Aborting preflight checks...", preflightLogFilename)
		return err
	}
	defer logFile.Close()
	logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
	logger.Infof("Created log file with name - %s", logFile.Name())
	lvl, err := log.ParseLevel(logLevel)
	if err != nil {
		logger.SetLevel(log.InfoLevel)
		logger.Errorf("Failed to parse log-level flag. Setting log level as %s\n", internal.DefaultLogLevel)
		return nil
	}
	logger.Infof("Setting log level as %s\n", logLevel)
	logger.SetLevel(lvl)

	return nil
}

func generateLogFileName(logFilePrefix string) string {
	year, month, day := time.Now().Date()
	hour, minute, sec := time.Now().Clock()
	ts := strconv.Itoa(year) + "-" + strconv.Itoa(int(month)) + "-" + strconv.Itoa(day) +
		"T" + strconv.Itoa(hour) + "-" + strconv.Itoa(minute) + "-" + strconv.Itoa(sec)

	return logFilePrefix + "-" + ts + ".log"
}

func logRootCmdFlagsInfo() {
	logger.Infof("Using '%s' namespace of the cluster", namespace)
	logger.Infof("Using kubeconfig file path - %s", kubeconfig)
}
