package sysutils

import (
	"fmt"
	"log"
	"os"
)

var (
	infoLogger    = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime)
	warningLogger = log.New(os.Stdout, "[WARNING] ", log.Ldate|log.Ltime)
	errorLogger   = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime)
	successLogger = log.New(os.Stdout, "[SUCCESS] ", log.Ldate|log.Ltime)
)

func Info(format string, v ...interface{}) {
	infoLogger.Output(2, fmt.Sprintf(format, v...))
}

func Warning(format string, v ...interface{}) {
	warningLogger.Output(2, fmt.Sprintf(format, v...))
}

func Error(format string, v ...interface{}) {
	errorLogger.Output(2, fmt.Sprintf(format, v...))
}

func Success(format string, v ...interface{}) {
	successLogger.Output(2, fmt.Sprintf(format, v...))
}
