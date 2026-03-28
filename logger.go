package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

const (
	LogLevel = logDbug
)

const (
	loggerSize = 2048
)

type logType int

const (
	logDbug logType = iota
	logInfo
	logWarn
	logErro
)

var (
	logList = make([]string, 0, loggerSize)
)

func printLog(t logType, msg interface{}) {
	if t < LogLevel {
		return
	}

	if len(logList) >= loggerSize {
		logList = logList[1:]
	}

	prefix := ""
	switch t {
	case logInfo:
		prefix = "INFO"
	case logWarn:
		prefix = "WARN"
	case logErro:
		prefix = "ERRO"
	}

	smsg := ""
	switch x := msg.(type) {
	case string:
		smsg = x
	case error:
		smsg = x.Error()
	}

	smsg = strings.ReplaceAll(smsg, "\n", ";")

	timeNow := time.Now().Format(time.DateTime)
	logList = append(logList, fmt.Sprintf("[%s] (%s) %s", prefix, timeNow, smsg))

	if scrollLoggerLabel == nil {
		return
	}

	scrollLoggerLabel.Content.(*fyne.Container).Objects[1].(*widget.Label).SetText(strings.Join(logList, "\n"))
	scrollLoggerLabel.ScrollToBottom()
}
