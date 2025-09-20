package types

import (
	"errors"
	"time"
)

// LOG LEVEL

type LogLevel string

const (
	Debug LogLevel = "DEBUG"
	Info  LogLevel = "INFO"
	Error LogLevel = "ERROR"
)

// LOG STRUCT

type LogEntry struct {
	Timestamp  int64 // unix timestamp
	Level      LogLevel
	Service    string
	Host       string
	Message    string
	StackTrace string
	Properties map[string]interface{} // key must be string, value can be dynamic
}

func NewLogEntry(level LogLevel, service, host, message string, stackTrace string, properties map[string]interface{}) (*LogEntry, error) {
	// check log level
	if !IsValidLogLevel(level) {
		return nil, errors.New("Invalid log level")
	}

	// make default properties dict
	if properties == nil {
		properties = make(map[string]interface{})
	}

	return &LogEntry{
		Timestamp:  time.Now().UnixMilli(),
		Level:      level,
		Service:    service,
		Host:       host,
		Message:    message,    // empty
		StackTrace: stackTrace, // empty
		Properties: properties,
	}, nil
}

// simple validaton for supported log level
func IsValidLogLevel(level LogLevel) bool {

	switch level {

	case Debug, Info, Error:
		return true

	}

	return false
}

// Properties helper

// can mutate the log entry
func (log *LogEntry) SetProperty(key string, value interface{}) {
	if log.Properties == nil {
		log.Properties = make(map[string]interface{})
	}

	log.Properties[key] = value
}

func (log *LogEntry) GetProperty(key string) (interface{}, bool) {
	value, ok := log.Properties[key]
	return value, ok
}
