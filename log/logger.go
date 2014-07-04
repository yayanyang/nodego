package logger


import (
	"log"
	"os"
	"strings"
)

//获取通用日志对象，最初由gonet项目实现
func GetLogger(path string) *log.Logger {

	if !strings.HasPrefix(path, "/") {
		path = os.Getenv("GOPATH") + "/" + path
	}

	// 打开文件
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println("error opening file %v\n", err)
		return nil
	}

	// 日志
	logger := log.New(file, "", log.LstdFlags)
	return logger
}