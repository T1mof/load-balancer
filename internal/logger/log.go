package logger

import (
	"io"
	"log"
	"os"
)

// LogLevel определяет уровень логирования
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// Logger предоставляет интерфейс для логирования
type Logger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
	fatal *log.Logger
	level LogLevel
}

// NewLogger создает новый логгер с настройками по умолчанию
func NewLogger() *Logger {
	return NewLoggerWithLevel(InfoLevel, os.Stdout)
}

// NewLoggerWithLevel создает новый логгер с указанным уровнем и выводом
func NewLoggerWithLevel(level LogLevel, output io.Writer) *Logger {
	flags := log.Ldate | log.Ltime

	return &Logger{
		debug: log.New(output, "\033[36mDEBUG\033[0m ", flags),
		info:  log.New(output, "\033[32mINFO\033[0m  ", flags),
		warn:  log.New(output, "\033[33mWARN\033[0m  ", flags),
		error: log.New(output, "\033[31mERROR\033[0m ", flags),
		fatal: log.New(output, "\033[35mFATAL\033[0m ", flags),
		level: level,
	}
}

// Debugf записывает debug сообщение с форматированием
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.debug.Printf(format, args...)
	}
}

// Debug записывает debug сообщение
func (l *Logger) Debug(args ...interface{}) {
	if l.level <= DebugLevel {
		l.debug.Println(args...)
	}
}

// Infof записывает info сообщение с форматированием
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.info.Printf(format, args...)
	}
}

// Info записывает info сообщение
func (l *Logger) Info(args ...interface{}) {
	if l.level <= InfoLevel {
		l.info.Println(args...)
	}
}

// Warnf записывает warning сообщение с форматированием
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.warn.Printf(format, args...)
	}
}

// Warn записывает warning сообщение
func (l *Logger) Warn(args ...interface{}) {
	if l.level <= WarnLevel {
		l.warn.Println(args...)
	}
}

// Errorf записывает error сообщение с форматированием
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.error.Printf(format, args...)
	}
}

// Error записывает error сообщение
func (l *Logger) Error(args ...interface{}) {
	if l.level <= ErrorLevel {
		l.error.Println(args...)
	}
}

// Fatalf записывает fatal сообщение с форматированием и завершает программу
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.fatal.Printf(format, args...)
	os.Exit(1)
}

// Fatal записывает fatal сообщение и завершает программу
func (l *Logger) Fatal(args ...interface{}) {
	l.fatal.Println(args...)
	os.Exit(1)
}

// SetLevel устанавливает уровень логирования
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}
