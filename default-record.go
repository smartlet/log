package log

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	skipBase = 3
)

const (
	slash = '/'
	space = ' '
	colon = ':'
	minus = '-'
)

var levels = [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
var digits = [...]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

type record struct {
	buffer *bytes.Buffer
	header [defaultHeaderLength]byte
	month  time.Month
	week   time.Weekday
	day    int
	hour   int
	minute int
}

// Header "2006/01/02 15:04:05 <LEVEL> "
func (r *record) Header(level Level) {
	t := time.Now()
	yr, mn, dy := t.Date()
	hr, mi, ss := t.Clock()

	r.hour = hr
	r.day = dy
	r.week = t.Weekday()
	r.month = mn

	// 格式化header: "yyyy/MM/dd HH:mm:ss [D] <file>:<line> - <message>"其中头部共24个字符
	r.header[0] = digits[yr/1000]
	yr %= 1000
	r.header[1] = digits[yr/100]
	yr %= 100
	r.header[2] = digits[yr/10]
	yr %= 10
	r.header[3] = digits[yr]
	r.header[4] = slash
	r.header[5] = digits[mn/10]
	r.header[6] = digits[mn%10]
	r.header[7] = slash
	r.header[8] = digits[dy/10]
	r.header[9] = digits[dy%10]
	r.header[10] = space
	r.header[11] = digits[hr/10]
	r.header[12] = digits[hr%10]
	r.header[13] = colon
	r.header[14] = digits[mi/10]
	r.header[15] = digits[mi%10]
	r.header[16] = colon
	r.header[17] = digits[ss/10]
	r.header[18] = digits[ss%10]
	r.header[19] = space

	switch level {
	case LevelDebug: // DEBUG
		r.header[20] = 'D'
		r.header[21] = 'E'
		r.header[22] = 'B'
		r.header[23] = 'U'
		r.header[24] = 'G'
		r.header[25] = space
		r.buffer.Write(r.header[:26])
	case LevelInfo: // INFO
		r.header[20] = 'I'
		r.header[21] = 'N'
		r.header[22] = 'F'
		r.header[23] = 'O'
		r.header[24] = space
		r.buffer.Write(r.header[:25])
	case LevelWarn: // WARN
		r.header[20] = 'W'
		r.header[21] = 'A'
		r.header[22] = 'R'
		r.header[23] = 'N'
		r.header[24] = space
		r.buffer.Write(r.header[:25])
	case LevelError: // ERROR
		r.header[20] = 'E'
		r.header[21] = 'R'
		r.header[22] = 'R'
		r.header[23] = 'O'
		r.header[24] = 'R'
		r.header[25] = space
		r.buffer.Write(r.header[:26])
	default: // <nil>
		r.header[20] = '<'
		r.header[21] = 'n'
		r.header[22] = 'i'
		r.header[23] = 'l'
		r.header[24] = '>'
		r.header[25] = space
		r.buffer.Write(r.header[:26])
	}
}

// Location ""
func (r *record) Location(skip int) {
	// 格式化file:line
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		file = "???"
		line = 1
	} else {
		// 最多截取后3个"/"开始的位置尾串
		num := 0
		for idx := len(file) - 1; idx >= 0; idx-- {
			if file[idx] == slash {
				num++
				if num == 3 {
					file = file[idx+1:]
					break
				}
			}
		}
	}
	r.buffer.WriteString(file)
	r.buffer.WriteByte(colon)
	r.buffer.WriteString(strconv.Itoa(line))
	r.buffer.WriteByte(space)
	r.buffer.WriteByte(minus)
	r.buffer.WriteByte(space)
}

func (r *record) Print(args ...interface{}) {
	fmt.Fprint(r.buffer, args...)
	r.buffer.WriteByte('\n')
}

func (r *record) Printf(format string, args ...interface{}) {
	fmt.Fprintf(r.buffer, format, args...)
	r.buffer.WriteByte('\n')
}

// PrintStack 打印堆栈追踪信息,如果是"/src/runtime/"自动跳过!
func (r *record) PrintStack(skip int) {
	for i := 1; ; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			return
		}
		// 过滤runtime的行项,避免错误日志过多!
		if strings.Index(file, "/src/runtime/") != -1 {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		r.buffer.WriteString(file)
		r.buffer.WriteByte(':')
		r.buffer.WriteString(strconv.Itoa(line))
		r.buffer.WriteByte('\n')
	}
}

func createRecords(recordBytes int, recordReuse int) *records {
	return &records{
		pool: sync.Pool{
			New: func() interface{} {
				return &record{
					buffer: bytes.NewBuffer(make([]byte, 0, recordBytes)),
				}
			},
		},
		threshold: recordBytes * recordReuse,
	}
}

type records struct {
	pool      sync.Pool
	threshold int
}

func (rs *records) Get() *record {
	r := rs.pool.Get().(*record)
	r.buffer.Reset()
	return r
}

func (rs *records) Put(r *record) {
	if r.buffer.Cap() < rs.threshold {
		rs.pool.Put(r)
	}
}
