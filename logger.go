package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

const (
	loggerSize = 2048
)

type logType int

const (
	logInfo logType = iota
	logWarn
	logErro
)

var (
	logList = make([]string, 0, loggerSize)
)

func printLog(t logType, msg interface{}) {
	if len(logList) >= loggerSize {
		logList = logList[1:]
	}

	prefix := ""
	switch t {
	case logInfo:
		prefix = "[INFO]"
	case logWarn:
		prefix = "[WARN]"
	case logErro:
		prefix = "[ERRO]"
	}

	smsg := ""
	switch x := msg.(type) {
	case string:
		smsg = x
	case error:
		smsg = x.Error()
	}

	smsg = strings.ReplaceAll(smsg, "\n", ";")
	logList = append(logList, fmt.Sprintf("%s: %s", prefix, smsg))

	if scrollLoggerLabel == nil {
		return
	}
	scrollLoggerLabel.Content.(*fyne.Container).Objects[1].(*widget.Label).SetText(strings.Join(logList, "\n"))
	scrollLoggerLabel.ScrollToBottom()
}
