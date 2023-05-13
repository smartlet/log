package log

# 功能

"短小精悍"的日志库. 定义统一的Logger接口, 提供类似glog的读写性能. 实用特性:

- 支持按大小轮转日志.
- 支持按周期轮转日志.
    - 月(monthly)
    - 周(weekly)
    - 日(daily)
    - 时(hourly)
- 支持异步写出日志.
- 支持discard舍弃超频日志,避免进程阻塞或磁盘损坏. 这是一种通过丢日志保稳定的行为!

# 用法

```
import "github.com/etabase/log"

log.Debug("msg1", "msg2", "msg3")
log.Debugf("%v -> %v -> %v", "msg1", "msg2", "msg3")
...

```

# 配置

```
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
```

