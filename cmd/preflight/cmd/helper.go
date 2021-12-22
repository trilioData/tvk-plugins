package cmd

func getLogLevelFromString(logLevelStr string) uint32 {
	switch logLevelStr {
	case logPanic:
		return logLevelPanic
	case logFatal:
		return logLevelFatal
	case logError:
		return logLevelError
	case logWarn:
		return logLevelWarn
	case logInfo:
		return logLevelInfo
	case logDebug:
		return logLevelDebug
	}

	return logLevelInfo
}
