package log

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
)

// brush is a color join function
type brush func(string) string

// newBrush return a fix color Brush
func newBrush(color string) brush {
	pre := "\033["
	reset := "\033[0m"
	return func(text string) string {
		return pre + color + "m" + text + reset
	}
}

var colors = []brush{
	newBrush("1;35"), // Fatal  magenta
	newBrush("1;31"), // Error  red
	newBrush("1;36"), // Alert	cyan
	newBrush("1;33"), // Warn   yellow
	newBrush("1;37"), // Notice	white
	newBrush("1;32"), // Info	green
	newBrush("1;34"), // Debug  blue
}

// consoleWriter implements LoggerInterface and writes messages to terminal.
type consoleWriter struct {
	lg    *log.Logger
	Level int `json:"level"`
}

// NewConsole create ConsoleWriter returning as LoggerInterface.
func NewConsole() Logger {
	cw := &consoleWriter{
		lg:    log.New(os.Stdout, "", log.Ldate|log.Ltime),
		Level: LevelDebug,
	}
	return cw
}

// Init init console logger.
// jsonconfig like '{"level":LevelTrace}'.
func (c *consoleWriter) Init(jsonconfig string) error {
	if len(jsonconfig) == 0 {
		return nil
	}
	return json.Unmarshal([]byte(jsonconfig), c)
}

// WriteMsg write message in console.
func (c *consoleWriter) WriteMsg(msg string, level int) error {
	if level > c.Level {
		return nil
	}
	if goos := runtime.GOOS; goos == "windows" {
		c.lg.Println(msg)
		return nil
	}
	c.lg.Println(colors[level](msg))

	return nil
}

// Destroy implementing method. empty.
func (c *consoleWriter) Destroy() {

}

// Flush implementing method. empty.
func (c *consoleWriter) Flush() {

}

func init() {
	Register("console", NewConsole)
}
