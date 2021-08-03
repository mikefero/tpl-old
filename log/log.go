package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Fields = logrus.Fields

var logger = logrus.New()

const tplLogPath = "tpl.log"

func init() {
	logger.Formatter = new(logrus.JSONFormatter)
	logger.Level = logrus.DebugLevel
	logger.Out = os.Stderr

	file, err := os.OpenFile(tplLogPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		logger.Out = file
	} else {
		logger.Error("Failed to log to file, using default stderr")
	}
}

func Trace(args ...interface{}) {
	logger.Trace(args...)
}
func Debug(args ...interface{}) {
	logger.Debug(args...)
}
func Info(args ...interface{}) {
	logger.Info(args...)
}
func Warn(args ...interface{}) {
	logger.Warn(args...)
}
func Error(args ...interface{}) {
	logger.Error(args...)
}
func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}
func Panic(args ...interface{}) {
	logger.Panic(args...)
}

func WithFields(fields Fields) *logrus.Entry {
	return logger.WithFields(fields)
}
