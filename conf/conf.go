package conf

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
)

func Get(filepath string, toPointer interface{}) (e error) {
	return ParseFile(filepath, toPointer, false)
}

func ParseFile(filepath string, toPointer interface{}, asScript bool) (e error) {
	if len(filepath) == 0 {
		e = errors.New("No conf file path")
	} else {
		var info os.FileInfo
		if info, e = os.Stat(filepath); e == nil {
			if info.IsDir() {
				e = errors.New("Not a file")
			} else {
				size := int(info.Size())
				buff := make([]byte, size)

				var f *os.File
				if f, e = os.OpenFile(filepath, os.O_RDONLY, 0666); e == nil {
					defer f.Close()
					var rc int
					if rc, e = f.Read(buff); e == nil {
						if rc == size {
							e = Parse(buff, toPointer, asScript)
						} else {
							e = errors.New("Read fail size")
						}
					}
				}
			}
		}
	}
	return
}

func Parse(confBytes []byte, toPointer interface{}, asScript bool) (e error) {
	content := string(confBytes)
	if asScript {
		if content, e = parseScriptCode(content); e != nil {
			return
		}
	}
	content = throwNote(content)
	if e = json.Unmarshal([]byte(content), toPointer); e != nil {
		e = errors.New("Cannot unmarshal below text as a json:\n" + content + "\nError:" + e.Error())
	}
	return
}

func throwNote(code string) string {
	lines := strings.Split(code, "\n")

	var runes []rune
	var lineLen int
	var cursor int
	var word rune
	var isInQuote bool

_FOR_LINES_:
	for lineIndex, line := range lines {
		// 从左边找到右边，如果先找到双引号，就找下一个双引号；如果先找到了//，就丢掉后面的
		runes = []rune(line)
		lineLen = len(runes)
		if lineLen == 0 {
			continue _FOR_LINES_
		}
		isInQuote = false

		for cursor, word = range runes[:lineLen-1] {
			if isInQuote { // 在String内
				if word == '"' {
					isInQuote = false
				}
			} else { // 不在String内
				switch word {
				case '"': // 进入了String
					isInQuote = true
				case '/':
					if runes[cursor+1] == '/' { // 下一个也是/，坐实了 //
						lines[lineIndex] = string(runes[:cursor])
						continue _FOR_LINES_
					}
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

var (
	_script_code_fmt_finder = regexp.MustCompile("`\\s{0,}`\\s{0,}`([.\\s\\S]+?)`\\s{0,}`\\s{0,}`")
	// _script_code_fmt_finder  = regexp.MustCompile("```([.\\s\\S]+?)```")
	_script_code_fmt_checker = regexp.MustCompile(`[^\\]"`)
)

func parseScriptCode(code string) (parsed string, e error) {
	indexes := _script_code_fmt_finder.FindAllStringSubmatchIndex(code, -1)
	count := len(indexes)
	if count == 0 {
		return code, nil
	}
	parsed = code[:indexes[0][0]]

	var innerText string
	for _i, ind := range indexes {
		// 0: start
		// 1: end
		// 2: inner start
		// 3: inner end
		innerText = code[ind[2]:ind[3]]
		if len(_script_code_fmt_checker.FindString(innerText)) != 0 {
			e = errors.New("脚本类型在使用```符号时，内部不能直接使用\"符号")
			return
		}
		parsed += `"` + strings.Replace(innerText, "\n", `\n`, -1) + `"`
		if _i+1 != count {
			parsed += code[ind[1]:indexes[_i+1][0]]
		}
	}

	parsed += code[indexes[count-1][1]:]

	return
}
