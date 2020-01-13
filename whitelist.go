package gocloc

import (
	"bufio"
	"fmt"
	"os"
)

type WhitelistResult struct {
	FileName     string `json:"file_name"`
	CodeLine     []int  `json:"code_line"`
	CommentsLine []int  `json:"comments_line"`
	BlanksLine   []int  `json:"blanks_line"`
}

func ReadWhitelistFiles(fileName string, v *[]string) {

	fin, err := os.OpenFile(fileName, os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	defer fin.Close()

	sc := bufio.NewScanner(fin)
	for sc.Scan() {
		*v = append(*v, sc.Text())
	}
	if err := sc.Err(); err != nil {
		fmt.Printf("An err has appeared")
	}
}

func WriteWhitelistResult() {

}
