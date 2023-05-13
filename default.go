package log

var DefaultLogger, _ = NewFileLogger(new(FileConfig))

func ResetDefaultLogger(c *FileConfig) error {
	if DefaultLogger != nil {
		DefaultLogger.Flush()
	}

	if lgr, err := NewFileLogger(c); err != nil {
		return err
	} else {
		DefaultLogger = lgr
	}

	return nil
}

func Debug(args ...interface{}) {
	DefaultLogger.Debug(args...)
}
func Debugf(format string, args ...interface{}) {
	DefaultLogger.Debugf(format, args...)
}
func Info(args ...interface{}) {
	DefaultLogger.Info(args...)
}
func Infof(format string, args ...interface{}) {
	DefaultLogger.Infof(format, args...)
}
func Warn(args ...interface{}) {
	DefaultLogger.Warn(args...)
}
func Warnf(format string, args ...interface{}) {
	DefaultLogger.Warnf(format, args...)
}
func Error(args ...interface{}) {
	DefaultLogger.Error(args...)
}
func Errorf(format string, args ...interface{}) {
	DefaultLogger.Errorf(format, args...)
}
func ErrorStack(format string, args ...interface{}) {
	DefaultLogger.ErrorStack(format, args...)
}
func Flush() {
	DefaultLogger.Flush()
}
