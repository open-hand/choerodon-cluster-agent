package util

import (
	"bytes"
	"os"
	"regexp"
	"runtime"
	"strconv"
)

var reg = regexp.MustCompile(`v1\.(\d+)?\..*`)

func CheckFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

// 获取协程号
func GetGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

/**
比较版本号,小于15返回false，大于等于15返回true
*/
func CompareVersion(currentVersionStr string) bool {
	currentVersionNumber := getVersionNumber(currentVersionStr)
	if currentVersionNumber < 15 {
		return false
	}
	return true
}

func getVersionNumber(versionStr string) int {
	versionNumber := reg.FindStringSubmatch(versionStr)[1]
	number, _ := strconv.Atoi(versionNumber)
	return number
}
