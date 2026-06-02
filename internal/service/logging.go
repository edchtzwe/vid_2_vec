package service

import (
	"io"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// NewFileLogger returns a *log.Logger that writes to both a rotating log file
// and os.Stdout via io.MultiWriter, enabling 12-factor app logging (stdout for
// container log collectors, file for persistent local history).
// If the log file cannot be opened, it falls back to stdout-only logging.
func NewFileLogger(path string) *log.Logger {
	flags := log.LstdFlags | log.Lshortfile

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("WARNING: failed to open log file %s: %v — falling back to stdout only", path, err)
		return log.New(os.Stdout, "", flags)
	}
	f.Close()

	w := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    50, // megabytes
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
	}

	mw := io.MultiWriter(w, os.Stdout)
	return log.New(mw, "", flags)
}
