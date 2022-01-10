package cmd

import (
	"io"

	"github.com/onsi/ginkgo/reporters/stenographer/support/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/trilioData/tvk-plugins/tools/preflight"
)

func setupLogger(logFilename string) error {
	var err error
	logFile, err = preflight.CreateLoggingFile(logFilename)
	if err != nil {
		log.Errorf("Unable to create log file - %s. Aborting preflight checks...",
			logFilename+"-"+preflight.GetLogFileTimestamp()+".log")
		return err
	}
	logger.SetOutput(io.MultiWriter(colorable.NewColorableStdout(), logFile))
	lvl, err := log.ParseLevel(logLevel)
	if err != nil {
		logger.SetLevel(log.InfoLevel)
		logger.Errorf("Failed to parse log-level flag. Setting log level as %s\n", defaultLogLevel)
		return err
	}
	logger.Infof("Setting log level as %s\n", logLevel)
	logger.SetLevel(lvl)

	return nil
}
