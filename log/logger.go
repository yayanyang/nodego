package logger


import (
	"log"
	// "os"
)

type DevNull struct{}

func (DevNull) Write(p []byte) (int, error) {
	return len(p), nil
}

// var(
// 	TRACE *log.Logger = log.New(os.Stdout, "TRACE ", log.Ldate|log.Ltime|log.Lshortfile)
// 	WARN  *log.Logger = log.New(os.Stdout, "WARN ", log.Ldate|log.Ltime|log.Lshortfile)
// 	INFO  *log.Logger = log.New(os.Stdout, "INFO ", log.Ldate|log.Ltime|log.Lshortfile)
// 	ERROR *log.Logger = log.New(os.Stderr, "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)
// )


var(
	TRACE *log.Logger = log.New(new(DevNull), "TRACE ", log.Ldate|log.Ltime|log.Lshortfile)
	WARN  *log.Logger = log.New(new(DevNull), "WARN ", log.Ldate|log.Ltime|log.Lshortfile)
	INFO  *log.Logger = log.New(new(DevNull), "INFO ", log.Ldate|log.Ltime|log.Lshortfile)
	ERROR *log.Logger = log.New(new(DevNull), "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)
)
