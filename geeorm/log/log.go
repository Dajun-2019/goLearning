//支持日志分级（info、error、disabled），不同层级使用不同颜色
//标准库不支持打印文件名和行号，这里支持
//封装了log的方法，方便使用
package log

import (
	"log"
	"os"
	"sync"
)

//使用 log.Lshortfile 支持显示文件名和代码行号
//\033[31m[error]\033[0m 是控制台输出颜色的格式，31m 表示红色，34m 表示蓝色，[error] 是前缀，[0m 表示关闭颜色
var (
	//红色
	errorLog = log.New(os.Stderr, "\033[31m[error]\033[0m", log.LstdFlags|log.Lshortfile)
	//蓝色
	infoLog = log.New(os.Stdout, "\033[34m[info]\033[0m", log.LstdFlags|log.Lshortfile)
	loggers = []*log.Logger{errorLog, infoLog}
	mu      sync.Mutex
)

//log 方法
var (
	Info   = infoLog.Println
	Error  = errorLog.Println
	Errorf = errorLog.Printf
	Infof  = infoLog.Printf
)

const (
	//日志分级
	InfoLevel = iota
	ErrorLevel
	Disabled
)

func SetLevel(level int) {
	mu.Lock()
	defer mu.Unlock()
	for _, logger := range loggers {
		logger.SetOutput(os.Stdout)
	}
	if level > ErrorLevel {
		errorLog.SetOutput(os.Stdout)
	}
	if level > InfoLevel {
		infoLog.SetOutput(os.Stdout)
	}
}
