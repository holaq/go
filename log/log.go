package log

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// levels
const (
	DebugLevel = iota
	InfoLevel
	ErrorLevel
	FatalLevel
)

const (
	PrintDebugLevel = "[D] "
	PrintInfoLevel  = "[I] "
	PrintErrorLevel = "[E] "
	PrintFatalLevel = "[F] "
)

type Logger struct {
	level      int
	baseLogger *log.Logger
	baseFile   *os.File
	filename   string
	maxDays    int64
	curDay     int
}

func New(strLevel string, filename string, maxDays int64) (*Logger, error) {
	// level
	var level int
	switch strings.ToLower(strLevel) {
	case "debug":
		level = DebugLevel
	case "info":
		level = InfoLevel
	case "error":
		level = ErrorLevel
	case "fatal":
		level = FatalLevel
	default:
		return nil, errors.New("unknown level: " + strLevel)
	}

	// logger
	var baseLogger *log.Logger
	var baseFile *os.File
	var curDay int
	if filename != "" {
		fname := filename
		if maxDays > 0 {
			fname = filename + time.Now().Format(".2006.01.02")
		}

		file, err := createLogFile(fname)
		//		file, err := os.Create(fname)
		if err != nil {
			return nil, err
		}
		baseLogger = log.New(file, "", log.LstdFlags)
		baseFile = file
		curDay = time.Now().Day()
	} else {
		baseLogger = log.New(os.Stdout, "", log.LstdFlags)
	}

	// new
	logger := new(Logger)
	logger.level = level
	logger.baseLogger = baseLogger
	logger.baseFile = baseFile
	logger.filename = filename
	logger.maxDays = maxDays
	logger.curDay = curDay
	return logger, nil
}

// It's dangerous to call the method on logging
func (logger *Logger) Close() {
	if logger.baseFile != nil {
		logger.baseFile.Close()
	}

	logger.baseLogger = nil
	logger.baseFile = nil
}

func (logger *Logger) doPrintf(level int, printLevel string, format string, a ...interface{}) {
	if level < logger.level {
		return
	}
	if logger.baseLogger == nil {
		panic("logger closed")
	}
	logger.doRotate()
	if level != InfoLevel {
		_, file, line, ok := runtime.Caller(2)
		if _, filename := path.Split(file); filename == "log.go" && (line == 222 || line == 226 || line == 230 || line == 234) {
			_, file, line, ok = runtime.Caller(3)
		}
		if ok {
			_, filename := path.Split(file)
			printLevel = fmt.Sprintf("[%s:%d] ", filename, line) + printLevel
		}

		format = printLevel + format
	}
	logger.baseLogger.Printf(format, a...)
	go logger.deleteOldLog()

	if level == FatalLevel {
		os.Exit(1)
	}
}

func createLogFile(filename string) (*os.File, error) {
	filePath := filepath.Dir(filename)
	if !fileExists(filePath) {
		os.MkdirAll(filePath, 0755)
	}
	// Open the log file
	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	return fd, err
}

// fileExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

//delete old log
func (logger *Logger) deleteOldLog() {
	if logger.filename == "" {
		return
	}
	dir := filepath.Dir(logger.filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("Unable to delete old log '%s', error: %+v", path, r)
				fmt.Println(returnErr)
			}
		}()
		if err != nil {
			panic(err)
		}
		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-60*60*24*logger.maxDays) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(logger.filename)) {
				if err := os.Remove(path); err != nil {
					panic(err)
				}
			}
		}
		return
	})
}

//rotate log by day
func (logger *Logger) doRotate() {
	if logger.filename == "" {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Unable to rotate log %s error: %+v\n", logger.filename, r)
		}
	}()
	info, _ := logger.baseFile.Stat()
	if !info.IsDir() && logger.curDay != time.Now().Day() { //rotate
		fname := logger.filename + time.Now().Format(".2006.01.02")

		file, _ := createLogFile(fname)
		logger.baseFile.Close()

		logger.baseLogger = log.New(file, "", log.LstdFlags)
		logger.baseFile = file
		logger.curDay = time.Now().Day()
	}
}

func (logger *Logger) Debug(format string, a ...interface{}) {
	logger.doPrintf(DebugLevel, PrintDebugLevel, format, a...)
}

func (logger *Logger) Info(format string, a ...interface{}) {
	logger.doPrintf(InfoLevel, PrintInfoLevel, format, a...)
}

func (logger *Logger) Error(format string, a ...interface{}) {
	logger.doPrintf(ErrorLevel, PrintErrorLevel, format, a...)
}

func (logger *Logger) Fatal(format string, a ...interface{}) {
	logger.doPrintf(FatalLevel, PrintFatalLevel, format, a...)
}

var gLogger, _ = New("debug", "", 0)

// It's dangerous to call the method on logging
func Export(logger *Logger) {
	if logger != nil {
		gLogger = logger
	}
}

func Debug(format string, a ...interface{}) {
	gLogger.Debug(format, a...)
}

func Info(format string, a ...interface{}) {
	gLogger.Info(format, a...)
}

func Error(format string, a ...interface{}) {
	gLogger.Error(format, a...)
}

func Fatal(format string, a ...interface{}) {
	gLogger.Fatal(format, a...)
}

func Close() {
	gLogger.Close()
}

func StartDefault(file string) error {
	if logger, e := New("debug", file, 7); e == nil {
		Export(logger)
	} else {
		return e
	}
	return nil
}
