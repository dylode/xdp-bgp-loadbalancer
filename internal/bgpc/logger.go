package bgpc

import (
	"github.com/charmbracelet/log"

	gobgpLog "github.com/osrg/gobgp/v3/pkg/log"
)

type GoBGPLogger struct{}

func convertFields(fields gobgpLog.Fields) []any {
	params := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		params = append(params, key, value)
	}
	return params
}

func (GoBGPLogger) Panic(msg string, fields gobgpLog.Fields) {
	log.Fatal(msg, convertFields(fields)...)
}

func (GoBGPLogger) Fatal(msg string, fields gobgpLog.Fields) {
	log.Fatal(msg, convertFields(fields)...)
}

func (GoBGPLogger) Error(msg string, fields gobgpLog.Fields) {
	log.Error(msg, convertFields(fields)...)
}

func (GoBGPLogger) Warn(msg string, fields gobgpLog.Fields) {
	log.Warn(msg, convertFields(fields)...)
}

func (GoBGPLogger) Info(msg string, fields gobgpLog.Fields) {
	log.Info(msg, convertFields(fields)...)
}

func (GoBGPLogger) Debug(msg string, fields gobgpLog.Fields) {
	log.Debug(msg, convertFields(fields)...)
}

func (GoBGPLogger) SetLevel(level gobgpLog.LogLevel) {
	switch level {
	case gobgpLog.FatalLevel:
	case gobgpLog.PanicLevel:
		log.SetLevel(log.FatalLevel)
	case gobgpLog.ErrorLevel:
		log.SetLevel(log.ErrorLevel)
	case gobgpLog.InfoLevel:
		log.SetLevel(log.InfoLevel)
	case gobgpLog.WarnLevel:
		log.SetLevel(log.WarnLevel)
	case gobgpLog.DebugLevel:
	case gobgpLog.TraceLevel:
		log.SetLevel(log.DebugLevel)
	}
}

func (GoBGPLogger) GetLevel() gobgpLog.LogLevel {
	switch log.GetLevel() {
	case log.FatalLevel:
		return gobgpLog.FatalLevel
	case log.ErrorLevel:
		return gobgpLog.ErrorLevel
	case log.WarnLevel:
		return gobgpLog.WarnLevel
	case log.DebugLevel:
		return gobgpLog.DebugLevel
	}

	return gobgpLog.InfoLevel
}
