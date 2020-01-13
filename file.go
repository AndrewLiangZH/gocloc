package gocloc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type ClocFile struct {
	Code     int32      `xml:"code,attr" json:"code"`
	Comments int32      `xml:"comment,attr" json:"comment"`
	Blanks   int32      `xml:"blank,attr" json:"blank"`
	Name     string     `xml:"name,attr" json:"name"`
	Lang     string     `xml:"language,attr" json"language"`
	LineNum  LineNumber `xml:"line_num,attr" json:"line_num"`
}

type LineNumber struct {
	CodeLine     []int `xml:"code_line,attr" json:"code_line"`
	CommentsLine []int `xml:"comments_line,attr" json:"comments_line"`
	BlanksLine   []int `xml:"blanks_line,attr" json:"blanks_line"`
}

type ClocFiles []ClocFile

func (cf ClocFiles) Len() int {
	return len(cf)
}
func (cf ClocFiles) Swap(i, j int) {
	cf[i], cf[j] = cf[j], cf[i]
}
func (cf ClocFiles) Less(i, j int) bool {
	if cf[i].Code == cf[j].Code {
		return cf[i].Name < cf[j].Name
	}
	return cf[i].Code > cf[j].Code
}

func AnalyzeFile(filename string, language *Language, opts *ClocOptions) *ClocFile {
	fp, err := os.Open(filename)
	if err != nil {
		// ignore error
		return &ClocFile{Name: filename}
	}
	defer fp.Close()

	return AnalyzeReader(filename, language, fp, opts)
}

func AnalyzeReader(filename string, language *Language, file io.Reader, opts *ClocOptions) *ClocFile {
	if opts.Debug {
		fmt.Printf("filename=%v\n", filename)
	}

	clocFile := &ClocFile{
		Name: filename,
		Lang: language.Name,
	}

	lineNum := 0
	isFirstLine := true
	inComments := [][2]string{}
	buf := getByteSlice()
	defer putByteSlice(buf)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(buf, 1024*1024)

scannerloop:
	for scanner.Scan() {
		lineNum++
		lineOrg := scanner.Text()
		line := strings.TrimSpace(lineOrg)

		if len(strings.TrimSpace(line)) == 0 {
			onBlank(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
			continue
		}

		// shebang line is 'code'
		if isFirstLine && strings.HasPrefix(line, "#!") {
			onCode(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
			isFirstLine = false
			continue
		}

		if len(inComments) == 0 {
			if isFirstLine {
				line = trimBOM(line)
			}

		singleloop:
			for _, singleComment := range language.lineComments {
				if strings.HasPrefix(line, singleComment) {
					// check if single comment is a prefix of multi comment
					for _, ml := range language.multiLines {
						if ml[0] != "" && strings.HasPrefix(line, ml[0]) {
							break singleloop
						}
					}
					onComment(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
					continue scannerloop
				}
			}

			if len(language.multiLines) == 0 {
				onCode(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
				continue scannerloop
			}
		}

		if len(inComments) == 0 && !containsComment(line, language.multiLines) {
			onCode(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
			continue scannerloop
		}

		isCode := false
		lenLine := len(line)
		if len(language.multiLines) == 1 && len(language.multiLines[0]) == 2 && language.multiLines[0][0] == "" {
			onCode(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
			continue
		}
		for pos := 0; pos < lenLine; {
			for _, ml := range language.multiLines {
				begin, end := ml[0], ml[1]
				lenBegin := len(begin)

				if pos+lenBegin <= lenLine && strings.HasPrefix(line[pos:], begin) && (begin != end || len(inComments) == 0) {
					pos += lenBegin
					inComments = append(inComments, [2]string{begin, end})
					continue
				}

				if n := len(inComments); n > 0 {
					last := inComments[n-1]
					if pos+len(last[1]) <= lenLine && strings.HasPrefix(line[pos:], last[1]) {
						inComments = inComments[:n-1]
						pos += len(last[1])
					}
				} else if pos < lenLine && !unicode.IsSpace(nextRune(line[pos:])) {
					isCode = true
				}
			}
			pos++
		}

		if isCode {
			onCode(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
		} else {
			onComment(clocFile, opts, len(inComments) > 0, line, lineOrg, lineNum)
		}
	}

	if opts.Debug {
		fmt.Printf("================================\n")
		fmt.Printf("code_line=%v\n", clocFile.LineNum.CodeLine)
		fmt.Printf("blanks_line=%v\n", clocFile.LineNum.BlanksLine)
		fmt.Printf("comments_line=%v\n", clocFile.LineNum.CommentsLine)
		fmt.Printf("================================\n")
	}

	return clocFile
}

func onBlank(clocFile *ClocFile, opts *ClocOptions, isInComments bool, line, lineOrg string, lineNum int) {
	clocFile.Blanks++
	clocFile.LineNum.BlanksLine = append(clocFile.LineNum.BlanksLine, lineNum)
	if opts.OnBlank != nil {
		opts.OnBlank(line)
	}

	if opts.Debug {
		fmt.Printf("[BLNK, cd:%d, cm:%d, bk:%d, iscm:%v, line_num:%d] %s\n",
			clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineNum, lineOrg)
	}
}

func onComment(clocFile *ClocFile, opts *ClocOptions, isInComments bool, line, lineOrg string, lineNum int) {
	clocFile.Comments++
	clocFile.LineNum.CommentsLine = append(clocFile.LineNum.CommentsLine, lineNum)
	if opts.OnComment != nil {
		opts.OnComment(line)
	}

	if opts.Debug {
		fmt.Printf("[COMM, cd:%d, cm:%d, bk:%d, iscm:%v, line_num:%d] %s\n",
			clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineNum, lineOrg)
	}
}

func onCode(clocFile *ClocFile, opts *ClocOptions, isInComments bool, line, lineOrg string, lineNum int) {
	clocFile.Code++
	clocFile.LineNum.CodeLine = append(clocFile.LineNum.CodeLine, lineNum)
	if opts.OnCode != nil {
		opts.OnCode(line)
	}

	if opts.Debug {
		fmt.Printf("[CODE, cd:%d, cm:%d, bk:%d, iscm:%v, line_num:%d] %s\n",
			clocFile.Code, clocFile.Comments, clocFile.Blanks, isInComments, lineNum, lineOrg)
	}
}
