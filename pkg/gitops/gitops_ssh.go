package gitops

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
)

const sshKeyPath = "/ssh-keys"

// 这个变量保存需要被替换的ssh地址
var SshRewriteUrlMap = make(map[string]string)
var sshProxyMap = make(map[string]string)

func init() {
	rawOriginSshUrl := os.Getenv("ORIGIN_SSH_URL")
	rawOverrideSshUrl := os.Getenv("REWRITE_SSH_URL")
	if len(rawOriginSshUrl) == 0 || len(rawOverrideSshUrl) == 0 {
		if sshProxy, exist := os.LookupEnv("SSH_PROXY"); exist && sshProxy != "" {
			url, err := url.Parse(sshProxy)
			if err != nil {
				glog.Error(err.Error())
				os.Exit(1)
			}
			sshProxyMap["schema"] = url.Scheme
			if url.User != nil {
				sshProxyMap["username"] = url.User.Username()
				sshProxyMap["password"], _ = url.User.Password()
			}
			sshProxyMap["host"] = url.Host
		}
		return
	}

	originSshUrls := strings.Split(rawOriginSshUrl, ",")
	overrideSshUrls := strings.Split(rawOverrideSshUrl, ",")
	if len(originSshUrls) != len(originSshUrls) {
		return
	}
	for index, originSshUrl := range originSshUrls {
		SshRewriteUrlMap[originSshUrl] = overrideSshUrls[index]
	}
}

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
	if rewriteUrl, ok := SshRewriteUrlMap[host]; ok {
		glog.Infof("origin host %s has been rewrited to %s", host, rewriteUrl)
		host = rewriteUrl
	}

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
	if len(SshRewriteUrlMap) == 0 && len(sshProxyMap) != 0 {
		result = result + fmt.Sprintf("  ProxyCommand ncat --proxy-type %s --proxy-auth %s:%s --proxy %s %%h %%p\n",
			sshProxyMap["schema"],
			sshProxyMap["username"],
			sshProxyMap["password"],
			sshProxyMap["host"])
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
