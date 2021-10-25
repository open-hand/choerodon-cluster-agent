package gitops

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"io/ioutil"
	"os"
	"strings"
)

const sshKeyPath = "/ssh-keys"

func (g *GitOps) PrepareSSHKeys(envs []model.EnvParas, opts *commandutil.Opts) error {

	var (
		err       error
		sshConfig string
	)
	for _, envPara := range envs {

		//写入deploy key(也就是拉取gitlab仓库需要的ssh-key)
		if err := writeSSHkey(envPara.Namespace, envPara.GitRsaKey); err != nil {
			return err
		}

		sshConfig = sshConfig + config(g.GitHost, envPara.Namespace)

	}

	// 写入ssh config
	if err := writeSshConfig(sshConfig); err != nil {
		return err
	}

	return err
}

func writeSSHkey(fileName, key string) error {

	filename := sshKeyPath + "/rsa-" + fileName

	var f *os.File
	if util.CheckFileIsExist(filename) { //如果文件存在
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
		hostnamePort := strings.Split(host, ":")
		hostname := hostnamePort[0]
		port := hostnamePort[1]
		result = result + fmt.Sprintf("  HostName %s\n", hostname)
		result = result + fmt.Sprintf("  Port %s\n", port)
	} else {
		result = result + fmt.Sprintf("  HostName %s\n", host)
	}
	result = result + fmt.Sprintf("  StrictHostKeyChecking no\n")
	result = result + fmt.Sprintf("  UserKnownHostsFile /dev/null\n")
	result = result + fmt.Sprintf("  IdentityFile %s/rsa-%s\n", sshKeyPath, namespace)
	result = result + fmt.Sprintf("  LogLevel error\n")
	httpProxy := os.Getenv("HTTP_PROXY_URL")
	if httpProxy != "" {
		result = result + fmt.Sprintf("  ProxyCommand connect -H %s %s %s\n", httpProxy,"%h","%p")
	}
	return result
}

func writeSshConfig(content string) error {

	filename := "/etc/ssh/ssh_config"
	var f *os.File
	if util.CheckFileIsExist(filename) { //如果文件存在
		_ = os.Remove(filename)
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
