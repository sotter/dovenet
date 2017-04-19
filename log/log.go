package log

import (
	"log"
	"fmt"
	"os"
)

var std_log = log.New(os.Stderr, "", log.LstdFlags)

func SetLog(l *log.Logger) {
	std_log = l
}

func Println(v ...interface{}) {
	std_log.Output(2, fmt.Sprintln(v...))
}

func Print(v ...interface{}) {
	std_log.Output(2, fmt.Sprintln(v...))
}
