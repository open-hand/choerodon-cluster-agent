package util

import (
	"bytes"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/version"
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
func CompareVersion(currentK8sVersion *version.Info) bool {
	minorVersion, err := strconv.Atoi(currentK8sVersion.Minor)
	if err != nil {
		glog.Error("failed to get k8s minorVersion: %s", currentK8sVersion.GitVersion)
	}
	if minorVersion < 15 {
		return false
	}
	return true
}

func GetCertMangerCrdFilePath(currentK8sVersion *version.Info) string {
	minorVersion, err := strconv.Atoi(currentK8sVersion.Minor)
	if err != nil {
		glog.Error("failed to get k8s minorVersion: %s", currentK8sVersion.GitVersion)
	}
	if minorVersion < 15 {
		return "/choerodon/cert-manager-legacy.crds.yaml"
	} else if minorVersion < 22 {
		return "/choerodon/cert-manager.crds-1.15-1.21.yaml"
	} else {
		return "/choerodon/cert-manager.crds-1.22.yaml"
	}
}
