package worker

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"io/ioutil"
	"os"
	"strings"
)

func init() {
	registerCmdFunc(model.InitAgent, setRepos)
	registerCmdFunc(model.ReSyncAgent, reSync)
	registerCmdFunc(model.UpgradeCluster, upgrade)
	registerCmdFunc(model.EnvDelete, deleteEnv)
	registerCmdFunc(model.CreateEnv,addEnv)


}



func writeSSHkey(fileName, key string) error {

	//filename := "/" + fileName
	filename := "/Users/crcokitwood/" + fileName
	var f *os.File
	if checkFileIsExist(filename) { //如果文件存在
		os.Remove(filename)
	}
	f, err := os.OpenFile(filename, os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	f.Close()
	err = ioutil.WriteFile(filename, []byte(key), 0600) //写入文件(字符串)
	if err != nil {
		return err
	}
	return nil
}

func config(host, namespace string) string {

	var result string
	result = result + fmt.Sprintf("Host %s\n", namespace)
	if strings.Contains(host, ":") {
		hostnamePort := strings.Split(host,":")
		hostname := hostnamePort[0]
		port := hostnamePort[1]
		result = result + fmt.Sprintf("  HostName %s\n", hostname)
		result = result + fmt.Sprintf("  Port %s\n", port)
	} else {
		result = result + fmt.Sprintf("  HostName %s\n", host)
	}
	result = result + fmt.Sprintf("  StrictHostKeyChecking no\n")
	result = result + fmt.Sprintf("  UserKnownHostsFile /dev/null\n")
	//result = result + fmt.Sprintf("  IdentityFile /%s\n", namespace)
	result = result + fmt.Sprintf("  IdentityFile /Users/crcokitwood/%s\n", namespace)
	result = result + fmt.Sprintf("  LogLevel error\n")
	return result
}

func writeSshConfig(content string) error {

	//filename := "/etc/ssh/ssh_config"
	filename:= "/Users/crcokitwood/ssh_config"
	var f *os.File
	if checkFileIsExist(filename) { //如果文件存在
		os.Remove(filename)
	}
	f, err := os.OpenFile(filename, os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	f.Close()
	err = ioutil.WriteFile(filename, []byte(content), 0666) //写入文件(字符串)
	if err != nil {
		return err
	}
	return nil
}

func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}
