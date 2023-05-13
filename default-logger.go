package log

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Cycle uint8

const (
	CycleOff     Cycle = 0
	CycleHourly  Cycle = 1
	CycleDaily   Cycle = 2
	CycleWeekly  Cycle = 3
	CycleMonthly Cycle = 4
)

type Level uint8

const (
	LevelDebug Level = 0
	LevelInfo  Level = 1
	LevelWarn  Level = 2
	LevelError Level = 3
	LevelOff   Level = 4
)

const (
	defaultHeaderLength = 26 // "2006/01/02 15:04:05 ERROR "
	defaultBufferBytes  = 256 * 1024
	defaultBufferFlush  = 15 * time.Second
	defaultRecordLength = 1024
	defaultRecordFactor = 4
)

type FileConfig struct {
	File          string        `json:"file"`           // 日志文件. stdout|stderr|<file-path>,默认stdout.
	Level         Level         `json:"level"`          // 日志级别. 默认Debug
	RotateCycle   Cycle         `json:"rotate_cycle"`   // 轮转周期. hourly|daily|weekly|monthly|never,默认never
	RotateBytes   int64         `json:"rotate_bytes"`   // 轮转大小. 默认0.
	DaemonMaximum int           `json:"daemon_maximum"` // 写通道长度. 默认0同步写.
	DaemonDiscard bool          `json:"daemon_discard"` // 写通道舍弃. 默认false不舍弃!
	BufferBytes   int           `json:"buffer_bytes"`   // 写缓存大小. 默认256K
	BufferFlush   time.Duration `json:"buffer_flush"`   // 写刷新周期. 默认15s
	RecordLength  int           `json:"record_length"`  // 记录缓存大小. 默认1024字节
	RecordFactor  int           `json:"record_factor"`  // 记录缓存重用. 默认4,即小于N*RecordFactor可重用.
}

func mergeDefaultSetting(s *FileConfig) *FileConfig {
	ss := *s
	if ss.File == "" {
		ss.File = STDOUT
	}
	if ss.BufferBytes <= 0 {
		ss.BufferBytes = defaultBufferBytes
	}
	if ss.BufferFlush <= 0 {
		ss.BufferFlush = defaultBufferFlush
	}
	if ss.RecordLength <= 0 {
		ss.RecordLength = defaultRecordLength
	}
	if ss.RecordFactor <= 0 {
		ss.RecordFactor = defaultRecordFactor
	}

	return &ss
}

type fileLogger struct {
	c           *FileConfig
	mutex       sync.Mutex
	level       Level
	records     *records
	file        *os.File
	writer      *bufio.Writer
	daemon      chan *record
	rotate      bool
	rotateMonth time.Month
	rotateWeek  time.Weekday
	rotateDay   int
	rotateHour  int
	rotateSize  int64
	Write       func(r *record)
}

func NewFileLogger(c *FileConfig) (Logger, error) {

	file, err := ToFile(c.File)
	if err != nil {
		return nil, err
	}
	lgr := new(fileLogger)
	lgr.c = mergeDefaultSetting(c)
	lgr.level = c.Level
	lgr.records = createRecords(lgr.c.RecordLength, lgr.c.RecordFactor)
	lgr.file = file
	lgr.writer = bufio.NewWriterSize(file, lgr.c.BufferBytes)
	lgr.rotate = file != os.Stdout && file != os.Stderr && (lgr.c.RotateBytes > 0 || lgr.c.RotateCycle > 0)
	lgr.daemon = make(chan *record, c.DaemonMaximum)
	if c.DaemonMaximum == 0 {
		lgr.Write = lgr.WriteDirect
	} else {
		if c.DaemonDiscard {
			lgr.Write = lgr.WriteDiscard
		} else {
			lgr.Write = lgr.WriteDaemon
		}
	}
	go lgr.Daemon(lgr.c.BufferFlush)
	return lgr, nil
}

func (lgr *fileLogger) Debug(args ...interface{}) {
	if lgr.level <= LevelDebug {
		r := lgr.records.Get()
		r.Header(LevelDebug)
		r.Location(skipBase)
		r.Print(args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Debugf(format string, args ...interface{}) {
	if lgr.level <= LevelDebug {
		r := lgr.records.Get()
		r.Header(LevelDebug)
		r.Location(skipBase)
		r.Printf(format, args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Info(args ...interface{}) {
	if lgr.level <= LevelInfo {
		r := lgr.records.Get()
		r.Header(LevelInfo)
		r.Location(skipBase)
		r.Print(args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Infof(format string, args ...interface{}) {
	if lgr.level <= LevelInfo {
		r := lgr.records.Get()
		r.Header(LevelInfo)
		r.Location(skipBase)
		r.Printf(format, args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Warn(args ...interface{}) {
	if lgr.level <= LevelWarn {
		r := lgr.records.Get()
		r.Header(LevelWarn)
		r.Location(skipBase)
		r.Print(args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Warnf(format string, args ...interface{}) {
	if lgr.level <= LevelWarn {
		r := lgr.records.Get()
		r.Header(LevelWarn)
		r.Location(skipBase)
		r.Printf(format, args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Error(args ...interface{}) {
	if lgr.level <= LevelError {
		r := lgr.records.Get()
		r.Header(LevelError)
		r.Location(skipBase)
		r.Print(args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Errorf(format string, args ...interface{}) {
	if lgr.level <= LevelError {
		r := lgr.records.Get()
		r.Header(LevelError)
		r.Location(skipBase)
		r.Printf(format, args...)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) ErrorStack(format string, args ...interface{}) {
	if lgr.level <= LevelError {
		r := lgr.records.Get()
		r.Header(LevelError)
		r.Location(skipBase)
		r.Printf(format, args...)
		r.PrintStack(skipBase)
		lgr.Write(r)
	}
}

func (lgr *fileLogger) Daemon(tk time.Duration) {
	for {
		err := lgr.protectDaemon(tk)
		if err == os.ErrProcessDone {
			return
		}
	}
}

func (lgr *fileLogger) protectDaemon(tk time.Duration) error {
	defer func() {
		if prr := recover(); prr != nil {
			fmt.Fprintf(os.Stderr, "protect daemon panic: %v", prr)
		}
	}()

	ticker := time.NewTicker(tk)
	defer ticker.Stop()

	for {
		select {
		case rc, ok := <-lgr.daemon:
			if !ok {
				return os.ErrProcessDone
			}
			lgr.WriteDirect(rc)
		case <-ticker.C:
			lgr.Flush()
		}
	}

}

func (lgr *fileLogger) Flush() {
	lgr.mutex.Lock()
	defer lgr.mutex.Unlock()

	lgr.writer.Flush()
	lgr.file.Sync()
}

func (lgr *fileLogger) rotating() {
	// 基于当前时间戳生成文件后缀(重复反缀不停尝试)
	var suffix = time.Now().Format("06010215")
	var rotateFile = lgr.c.File + "." + suffix

	if sta, err := os.Stat(rotateFile); sta != nil || os.IsExist(err) {
		count := 0
		for {
			prefixFile := rotateFile + "." + strconv.Itoa(count)
			if sta, err = os.Stat(prefixFile); sta == nil || os.IsNotExist(err) {
				rotateFile = prefixFile
				break
			}
			count++
		}
	}

	lgr.writer.Flush()
	lgr.file.Close()
	os.Rename(lgr.c.File, rotateFile)

	if file, err := ToFile(lgr.c.File); err != nil {
		fmt.Fprintf(os.Stderr, "logger open file error: %v, %v", lgr.c.File, err)
	} else {
		lgr.file = file
		lgr.writer = bufio.NewWriterSize(file, lgr.c.BufferBytes)
		now := time.Now()
		lgr.rotateMonth = now.Month()
		lgr.rotateWeek = now.Weekday()
		lgr.rotateDay = now.Day()
		lgr.rotateHour = now.Hour()
		lgr.rotateSize = 0
	}
}

func (lgr *fileLogger) WriteDirect(r *record) {

	defer lgr.records.Put(r)

	lgr.mutex.Lock()
	defer lgr.mutex.Unlock()

	if lgr.rotate {
		if lgr.c.RotateCycle > CycleOff {
			switch lgr.c.RotateCycle {
			case CycleMonthly:
				if r.month != lgr.rotateMonth {
					lgr.rotating()
					goto __write__
				}
			case CycleWeekly:
				if r.month != lgr.rotateMonth || r.week != lgr.rotateWeek {
					lgr.rotating()
					goto __write__
				}
			case CycleDaily:
				if r.month != lgr.rotateMonth || r.week != lgr.rotateWeek || r.day != lgr.rotateDay {
					lgr.rotating()
					goto __write__
				}
			case CycleHourly:
				if r.month != lgr.rotateMonth || r.week != lgr.rotateWeek || r.day != lgr.rotateDay || r.hour != lgr.rotateHour {
					lgr.rotating()
					goto __write__
				}
			}
		}
		if lgr.c.RotateBytes > 0 {
			size := int64(r.buffer.Len())
			if size+lgr.rotateSize > lgr.c.RotateBytes {
				lgr.rotating()
				goto __write__
			}
			lgr.rotateSize += size
		}
	}
__write__:
	_, err := lgr.writer.Write(r.buffer.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger write error: %v", err)
	}
}

func (lgr *fileLogger) WriteDaemon(r *record) {
	lgr.daemon <- r
}

func (lgr *fileLogger) WriteDiscard(r *record) {
	select {
	case lgr.daemon <- r:
		// write
	default:
		// discard
	}
}

var _ Logger = (*fileLogger)(nil)

const (
	STDOUT  = "stdout"  // 标准输出流(不区分大小写)
	STDERR  = "stderr"  // 标准错误流(不区分大小写)
	DEBUG   = "debug"   // 调试级别(不区分大小写)
	INFO    = "info"    // 信息级别(不区分大小写)
	WARN    = "warn"    // 警告级别(不区分大小写)
	ERROR   = "error"   // 错误级别(不区分大小写)
	OFF     = "off"     // 忽略此项(不区分大小写)
	HOURLY  = "hourly"  // 时周期(不区分大小写)
	DAILY   = "daily"   // 日周期(不区分大小写)
	WEEKLY  = "weekly"  // 周周期(不区分大小写)
	MONTHLY = "monthly" // 月周期(不区分大小写)

)

func ToFile(path string) (file *os.File, err error) {

	switch strings.ToLower(path) {
	case STDOUT:
		file = os.Stdout
	case STDERR:
		file = os.Stderr
	default:
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return
		}
	}
	return
}

func ToLevel(level string) (Level, error) {
	switch strings.ToLower(level) {
	case DEBUG:
		return LevelDebug, nil
	case INFO:
		return LevelInfo, nil
	case WARN:
		return LevelWarn, nil
	case ERROR:
		return LevelError, nil
	case OFF:
		return LevelOff, nil
	}
	return 0, fmt.Errorf("invalid level value: %v", level)
}

func ToCycle(cycle string) (Cycle, error) {
	switch strings.ToLower(cycle) {
	case HOURLY:
		return CycleHourly, nil
	case DAILY:
		return CycleDaily, nil
	case WEEKLY:
		return CycleWeekly, nil
	case MONTHLY:
		return CycleMonthly, nil
	case OFF:
		return CycleOff, nil
	}
	return 0, fmt.Errorf("invalid cycle value: %v", cycle)
}
