package worker

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"io/ioutil"
	"os"
	"strings"
)

func init() {
	registerCmdFunc(model.InitAgent, setRepos)
}


func initAgent(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var agentInitOps model.AgentInitOptions
	err := json.Unmarshal([]byte(cmd.Payload), &agentInitOps)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	var sshConfig string

	// 往文件中写入各个git库deploy key
	for _,envPara := range agentInitOps.Envs {
	    err = writeSSHkey(envPara.Namespace, envPara.GitRsaKey)
	    if err != nil {
	    	return nil, NewResponseError(cmd.Key, model.InitAgentFailed, err)
		}
		sshConfig = sshConfig + config(agentInitOps.GitHost, envPara.Namespace)
	}

	err = writeSshConfig(sshConfig)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	return nil, &model.Packet{
		Key:     "test:test",
		Type:    model.InitAgentSucceed,
		Payload: cmd.Payload,
	}
}

func writeSSHkey(fileName, key string) error {

	filename := "/" + fileName
	//filename := "/Users/crcokitwood/" + fileName
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
		port = result + fmt.Sprintf("  Port %s\n", port)
	} else {
		result = result + fmt.Sprintf("  HostName %s\n", host)
	}
	result = result + fmt.Sprintf("  StrictHostKeyChecking no\n")
	result = result + fmt.Sprintf("  UserKnownHostsFile /dev/null\n")
	result = result + fmt.Sprintf("  IdentityFile /%s\n", namespace)
	//result = result + fmt.Sprintf("  IdentityFile /Users/crcokitwood/%s\n", namespace)
	result = result + fmt.Sprintf("  LogLevel error\n")
	return result
}

func writeSshConfig(content string) error {

	filename := "/etc/ssh/ssh_config"
	//filename:= "/Users/crcokitwood/ssh_config"
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
