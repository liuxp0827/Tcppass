package log

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// RFC5424 log message levels.
const (
	LevelFatal = iota
	LevelError
	LevelAlert
	LevelWarn
	LevelNotice
	LevelInfo
	LevelDebug
	LevelTrace
)

type loggerType func() Logger

// Logger defines the behavior of a log provider.
type Logger interface {
	Init(config string) error
	WriteMsg(msg string, level int) error
	Destroy()
	Flush()
}

var adapters = make(map[string]loggerType)

// Register makes a log provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func Register(name string, log loggerType) {
	if log == nil {
		panic("logs: Register provide is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}

// ZeusLogger is default logger in beego application.
// it can contain several providers and log message into all providers.
type ZeusLogger struct {
	lock                sync.Mutex
	level               int
	enableFuncCallDepth bool
	loggerFuncCallDepth int
	asynchronous        bool
	msgChan             chan *logMsg
	outputs             []*nameLogger
}

type nameLogger struct {
	Logger
	name string
}

type logMsg struct {
	level int
	msg   string
}

var logMsgPool *sync.Pool

// NewLogger returns a new ZeusLogger.
// channelLen means the number of messages in chan(used where asynchronous is true).
// if the buffering chan is full, logger adapters write to file or other way.
func NewLogger(channelLen int64) *ZeusLogger {
	bl := new(ZeusLogger)
	bl.level = LevelDebug
	bl.loggerFuncCallDepth = 2
	bl.msgChan = make(chan *logMsg, channelLen)
	return bl
}

// Async set the log to asynchronous and start the goroutine
func (bl *ZeusLogger) Async() *ZeusLogger {
	bl.asynchronous = true
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &logMsg{}
		},
	}
	go bl.startLogger()
	return bl
}

// SetLogger provides a given logger adapter into ZeusLogger with config string.
// config need to be correct JSON as string: {"interval":360}.
func (bl *ZeusLogger) SetLogger(adapterName string, config string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if log, ok := adapters[adapterName]; ok {
		lg := log()

		err := lg.Init(config)
		if err != nil {
			fmt.Fprintln(os.Stderr, "logs.ZeusLogger.SetLogger: "+err.Error())
			return err
		}
		bl.outputs = append(bl.outputs, &nameLogger{name: adapterName, Logger: lg})
	} else {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	return nil
}

func (bl *ZeusLogger) SetLogFile(logFile string, level int, isRotateDaily, drawColor bool, rotateMaxDays int) error {
	return bl.SetLogger("file", fmt.Sprintf(`{"filename":"%s","level":%d,"daily":%v,"maxdays":%d,"drawcolor":%v}`,
		logFile, level, isRotateDaily, rotateMaxDays, drawColor))
}

// DelLogger remove a logger adapter in ZeusLogger.
func (bl *ZeusLogger) DelLogger(adapterName string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	outputs := []*nameLogger{}
	for _, lg := range bl.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(bl.outputs) {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	bl.outputs = outputs
	return nil
}

func (bl *ZeusLogger) writeToLoggers(msg string, level int) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(msg, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

func (bl *ZeusLogger) writeMsg(logLevel int, msg string) error {
	if bl.enableFuncCallDepth {
		_, file, line, ok := runtime.Caller(bl.loggerFuncCallDepth)
		if !ok {
			file = "???"
			line = 0
		}
		_, filename := path.Split(file)
		msg = "[" + filename + ":" + strconv.FormatInt(int64(line), 10) + "]" + msg
	}
	if bl.asynchronous {
		lm := logMsgPool.Get().(*logMsg)
		lm.level = logLevel
		lm.msg = msg
		bl.msgChan <- lm
	} else {
		bl.writeToLoggers(msg, logLevel)
	}
	return nil
}

// SetLevel Set log message level.
// If message level (such as LevelDebug) is higher than logger level (such as LevelWarn),
// log providers will not even be sent the message.
func (bl *ZeusLogger) SetLevel(l int) {
	bl.level = l
}

// SetLogFuncCallDepth set log funcCallDepth
func (bl *ZeusLogger) SetLogFuncCallDepth(d int) {
	bl.loggerFuncCallDepth = d
}

// GetLogFuncCallDepth return log funcCallDepth for wrapper
func (bl *ZeusLogger) GetLogFuncCallDepth() int {
	return bl.loggerFuncCallDepth
}

// EnableFuncCallDepth enable log funcCallDepth
func (bl *ZeusLogger) EnableFuncCallDepth(b bool) {
	bl.enableFuncCallDepth = b
}

func (bl *ZeusLogger) SetLogFuncCallWithDepth(b bool, depth int) {
	bl.EnableFuncCallDepth(b)
	bl.SetLogFuncCallDepth(depth)
}

func (bl *ZeusLogger) SetLogFuncCall(b bool) {
	bl.EnableFuncCallDepth(b)
	bl.SetLogFuncCallDepth(3)
}

// start logger chan reading.
// when chan is not empty, write logs.
func (bl *ZeusLogger) startLogger() {
	for {
		select {
		case bm := <-bl.msgChan:
			bl.writeToLoggers(bm.msg, bm.level)
			logMsgPool.Put(bm)
		}
	}
}

// Emergency Log FATAL level message.
func (bl *ZeusLogger) Fatalf(format string, v ...interface{}) {
	if LevelFatal > bl.level {
		return
	}
	msg := fmt.Sprintf("[F] "+format, v...)
	bl.writeMsg(LevelFatal, msg)
	os.Exit(2)
}

func (bl *ZeusLogger) Fatal(v ...interface{}) {
	if LevelFatal > bl.level {
		return
	}
	msg := fmt.Sprintf("[F] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelFatal, msg)
	os.Exit(2)
}

// Alert Log ALERT level message.
func (bl *ZeusLogger) Alertf(format string, v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}
	msg := fmt.Sprintf("[A] "+format, v...)
	bl.writeMsg(LevelAlert, msg)
}

func (bl *ZeusLogger) Alert(v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}
	msg := fmt.Sprintf("[A] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelAlert, msg)
}

// Error Log ERROR level message.
func (bl *ZeusLogger) Errorf(format string, v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	msg := fmt.Sprintf("[E] "+format, v...)
	bl.writeMsg(LevelError, msg)
}

func (bl *ZeusLogger) Error(v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	msg := fmt.Sprintf("[E] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelError, msg)
}

// Warning Log WARNING level message.
func (bl *ZeusLogger) Warnf(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	msg := fmt.Sprintf("[W] "+format, v...)
	bl.writeMsg(LevelWarn, msg)
}

func (bl *ZeusLogger) Warn(v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	msg := fmt.Sprintf("[W] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelWarn, msg)
}

// Notice Log NOTICE level message.
func (bl *ZeusLogger) Noticef(format string, v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	msg := fmt.Sprintf("[N] "+format, v...)
	bl.writeMsg(LevelNotice, msg)
}

func (bl *ZeusLogger) Notice(v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	msg := fmt.Sprintf("[N] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelNotice, msg)
}

// Debug Log DEBUG level message.
func (bl *ZeusLogger) Debugf(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	msg := fmt.Sprintf("[D] "+format, v...)
	bl.writeMsg(LevelDebug, msg)
}

func (bl *ZeusLogger) Debug(v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	msg := fmt.Sprintf("[D] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelDebug, msg)
}

// Info Log INFO level message.
// compatibility alias for Informational()
func (bl *ZeusLogger) Infof(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	msg := fmt.Sprintf("[I] "+format, v...)
	bl.writeMsg(LevelInfo, msg)
}

func (bl *ZeusLogger) Info(v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	msg := fmt.Sprintf("[I] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelInfo, msg)
}

// Trace Log TRACE level message.
// compatibility alias for Debug()
func (bl *ZeusLogger) Tracef(format string, v ...interface{}) {
	if LevelTrace > bl.level {
		return
	}
	msg := fmt.Sprintf("[T] "+format, v...)
	bl.writeMsg(LevelDebug, msg)
}

func (bl *ZeusLogger) Trace(v ...interface{}) {
	if LevelTrace > bl.level {
		return
	}
	msg := fmt.Sprintf("[T] "+generateFmtStr(len(v)), v...)
	bl.writeMsg(LevelDebug, msg)
}

// Flush flush all chan data.
func (bl *ZeusLogger) Flush() {
	for _, l := range bl.outputs {
		l.Flush()
	}
}

// Close close logger, flush all chan data and destroy all adapters in ZeusLogger.
func (bl *ZeusLogger) Close() {
	for {
		if len(bl.msgChan) > 0 {
			bm := <-bl.msgChan
			bl.writeToLoggers(bm.msg, bm.level)
			logMsgPool.Put(bm)
			continue
		}
		break
	}
	for _, l := range bl.outputs {
		l.Flush()
		l.Destroy()
	}
}

func generateFmtStr(n int) string {
	return strings.Repeat("%v ", n)
}
