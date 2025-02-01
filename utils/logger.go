package utils

import (
	"fmt"
	"io"
	"os"
	"reflect"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	DefaultDebugLogPath = "debug.log"
)

type Logger struct {
	logPath string
}

func NewLogger(logPath string) *Logger {
	if logPath == "" {
		logPath = DefaultDebugLogPath
	}
	return &Logger{logPath: logPath}
}

func (l *Logger) Debug(message string) error {
	if os.Getenv("DEBUG") == "" {
		return nil
	}

	f, err := tea.LogToFile(l.logPath, "debug")
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	return nil
}

func (l *Logger) CleanLogFile() error {
	if _, err := os.Stat(l.logPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	if err := os.Truncate(l.logPath, 0); err != nil {
		return fmt.Errorf("failed to truncate log file: %w", err)
	}
	return nil
}

type StructPrinter struct {
	writer io.Writer
}

func NewStructPrinter(w io.Writer) *StructPrinter {
	if w == nil {
		w = os.Stdout
	}
	return &StructPrinter{writer: w}
}

func (p *StructPrinter) Print(v interface{}) error {
	if v == nil {
		return fmt.Errorf("nil interface provided")
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Slice {
		return p.printSlice(val)
	}

	return p.printStruct(val)
}

func (p *StructPrinter) printSlice(val reflect.Value) error {
	for i := 0; i < val.Len(); i++ {
		if err := p.Print(val.Index(i).Interface()); err != nil {
			return fmt.Errorf("failed to print slice element %d: %w", i, err)
		}
	}
	return nil
}

func (p *StructPrinter) printStruct(val reflect.Value) error {
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.IsValid() {
			continue
		}

		fieldName := typ.Field(i).Name
		_, err := fmt.Fprintf(p.writer, "%s: %v\n", fieldName, field.Interface())
		if err != nil {
			return fmt.Errorf("failed to print field %s: %w", fieldName, err)
		}
	}
	return nil
}
