package console

import (
	"fmt"
	"runtime"
)

const (
	_TextBlack = iota + 30
	_TextRed
	_TextGreen
	_TextYellow
	_TextBlue
	_TextMagenta
	_TextCyan
	_TextWhite
)

var (
	IsWindows = false
)

func init() {
	IsWindows = _isWindows()
}

func Black(str ...interface{}) string {
	return textColor(_TextBlack, str)
}

func Red(str ...interface{}) string {
	return textColor(_TextRed, str)
}

func Green(str ...interface{}) string {
	return textColor(_TextGreen, str)
}

func Yellow(str ...interface{}) string {
	return textColor(_TextYellow, str)
}

func Blue(str ...interface{}) string {
	return textColor(_TextBlue, str)
}

func Magenta(str ...interface{}) string {
	return textColor(_TextMagenta, str)
}

func Cyan(str ...interface{}) string {
	return textColor(_TextCyan, str)
}

func White(str ...interface{}) string {
	return textColor(_TextWhite, str)
}

func textColor(color int, args []interface{}) string {
	var str string
	if args == nil {
		return ""
	}
	argCount := len(args)
	if argCount == 0 {
		return ""
	}
	var isStr bool
	if str, isStr = args[0].(string); isStr {
		str = fmt.Sprintf(str, args[1:]...)
	} else {
		str = fmt.Sprint(args)
	}
	if IsWindows {
		return str
	}

	switch color {
	case _TextBlack:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextBlack, str)
	case _TextRed:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextRed, str)
	case _TextGreen:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextGreen, str)
	case _TextYellow:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextYellow, str)
	case _TextBlue:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextBlue, str)
	case _TextMagenta:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextMagenta, str)
	case _TextCyan:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextCyan, str)
	case _TextWhite:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", _TextWhite, str)
	default:
		return fmt.Sprintf("%v", str)
	}
}

func _isWindows() bool {
	return runtime.GOOS == "windows"
}
