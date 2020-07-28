package kubectl

import (
	"bytes"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	vlog "github.com/vinkdong/gox/log"
	"io"
	"k8s.io/client-go/rest"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Kubectl struct {
	exe    string
	config *rest.Config
}

func NewKubectl(exe string, config *rest.Config) *Kubectl {
	return &Kubectl{
		exe:    exe,
		config: config,
	}
}

func (c *Kubectl) connectArgs() []string {
	var args []string
	if c.config.Host != "" {
		args = append(args, fmt.Sprintf("--server=%s", c.config.Host))
	}
	if c.config.Username != "" {
		args = append(args, fmt.Sprintf("--username=%s", c.config.Username))
	}
	if c.config.Password != "" {
		args = append(args, fmt.Sprintf("--password=%s", c.config.Password))
	}
	if c.config.TLSClientConfig.CertFile != "" {
		args = append(args, fmt.Sprintf("--client-certificate=%s", c.config.TLSClientConfig.CertFile))
	}
	if c.config.TLSClientConfig.CAFile != "" {
		args = append(args, fmt.Sprintf("--certificate-authority=%s", c.config.TLSClientConfig.CAFile))
	}
	if c.config.TLSClientConfig.KeyFile != "" {
		args = append(args, fmt.Sprintf("--client-key=%s", c.config.TLSClientConfig.KeyFile))
	}
	if c.config.BearerToken != "" {
		args = append(args, fmt.Sprintf("--token=%s", c.config.BearerToken))
	}
	return args
}

func (c *Kubectl) Apply(namespace string, cs kubernetes.ChangeSet) (errs kubernetes.SyncError) {
	return c.apply(namespace, cs)
}

func (c *Kubectl) apply(namespace string, cs kubernetes.ChangeSet) (errs kubernetes.SyncError) {
	f := func(objs []*kubernetes.ApiObject, cmd string, args ...string) {
		if len(objs) == 0 {
			return
		}
		glog.Infof("cmd: %s, args: %s, count: %d", cmd, strings.Join(args, " "), len(objs))
		args = append(args, cmd, "-n", namespace, "--wait=false")
		if err := c.doCommand(makeMultidoc(objs), args...); err != nil {
			for _, obj := range objs {
				r := bytes.NewReader(obj.Bytes())
				if err := c.doCommand(r, args...); err != nil {
					errs = append(errs, kubernetes.ResourceError{obj.Resource, err})
				}
			}
		}
	}

	// When deleting objects, the only real concern is that we don't
	// try to delete things that have already been deleted by
	// Kubernete's GC -- most notably, resources in a namespace which
	// is also being deleted. GC does not have the dependency ranking,
	// but we can use it as a shortcut to avoid the above problem at
	// least.
	objs := cs.DeleteObj()
	sort.Sort(sort.Reverse(applyOrder(objs)))
	f(objs, "delete")

	objs = cs.ApplyObj()
	sort.Sort(applyOrder(objs))
	f(objs, "apply")
	return errs
}

func (c *Kubectl) ApplySingleObj(namespace string, resourceFile string) error {
	r := bytes.NewReader([]byte(resourceFile))
	err := c.doCommand(r, "apply")
	if err != nil {
		glog.Errorf("app k8s resource error %v\n%s\n", err, resourceFile)
		return err
	}
	return nil
}

func (c *Kubectl) DeletePrometheusCrd() error {
	args := []string{"delete", "crd", "prometheuses.monitoring.coreos.com", "prometheusrules.monitoring.coreos.com", "servicemonitors.monitoring.coreos.com", "podmonitors.monitoring.coreos.com", "alertmanagers.monitoring.coreos.com"}
	cmd := c.kubectlCommand(args...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout

	begin := time.Now()
	err := cmd.Run()
	glog.Infof("kubectl %s , took %v, err: %v, output: \n%s", strings.Join(args, " "), time.Since(begin), err, strings.TrimSpace(stdout.String()))
	if err != nil {
		err = errors.Wrap(errors.New(strings.TrimSpace(stderr.String())), "running kubectl")
		return err
	}
	return nil
}

func (c *Kubectl) Describe(namespace, sourceKind, sourceName string) (info string) {
	return c.describe(namespace, sourceKind, sourceName)
}

func (c *Kubectl) describe(namespace, sourceKind, sourceName string) (info string) {
	args := []string{"describe", sourceKind, sourceName}
	if len(namespace) != 0 {
		args = append(args, "-n", namespace)
	}
	cmd := c.kubectlCommand(args...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	begin := time.Now()
	err := cmd.Run()
	if err != nil {
		glog.Infof("kubectl %s , took %v, err: %v, output: \n%s", strings.Join(args, " "), time.Since(begin), err, strings.TrimSpace(stdout.String()))
		return strings.TrimSpace(stderr.String())
	}
	glog.Infof("kubectl %s , took %v, output: \n%s", strings.Join(args, " "), time.Since(begin), strings.TrimSpace(stdout.String()))
	return strings.TrimSpace(stdout.String())
}

func (c *Kubectl) Scaler(namespace, sourceKind, sourceName, replicas string) error {
	return c.scaler(namespace, sourceKind, sourceName, replicas)
}

func (c *Kubectl) scaler(namespace, sourceKind, sourceName, replicas string) error {
	args := []string{"scale", "--replicas", replicas, sourceKind, sourceName}
	if len(namespace) != 0 {
		args = append(args, "-n", namespace)
	}
	cmd := c.kubectlCommand(args...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	begin := time.Now()
	err := cmd.Run()
	if err != nil {
		glog.Infof("kubectl %s , took %v, err: %v, output: \n%s", strings.Join(args, " "), time.Since(begin), err, strings.TrimSpace(stdout.String()))
		return err
	}
	return nil
}

func (c *Kubectl) doCommand(r io.Reader, args ...string) error {
	args = append(args, "-f", "-")
	cmd := c.kubectlCommand(args...)
	cmd.Stdin = r
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout

	begin := time.Now()
	err := cmd.Run()
	if err != nil {
		err = errors.Wrap(errors.New(strings.TrimSpace(stderr.String())), "running kubectl")
	}

	glog.Infof("kubectl %s , took %v, err: %v, output: \n%s", strings.Join(args, " "), time.Since(begin), err, strings.TrimSpace(stdout.String()))
	return err
}

func (c *Kubectl) kubectlCommand(args ...string) *exec.Cmd {
	return exec.Command(c.exe, append(c.connectArgs(), args...)...)
}

func (c *Kubectl) kubectlCommandInShell(command string) *exec.Cmd {
	return exec.Command("sh", "-c", c.exe+" "+command)
}

func (c *Kubectl) Do(command string) (string, error) {
	return c.do(command)
}

func (c *Kubectl) do(command string) (string, error) {
	cmd := c.kubectlCommandInShell(command)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	begin := time.Now()
	err := cmd.Run()
	if err != nil {
		vlog.Infof("kubectl: %s , took %v, err: %v, output: %s", command, time.Since(begin), err, strings.TrimSpace(stderr.String()))
		return "", err
	}
	return stdout.String(), err
}

type applyOrder []*kubernetes.ApiObject

func (objs applyOrder) Len() int {
	return len(objs)
}

func (objs applyOrder) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

func (objs applyOrder) Less(i, j int) bool {
	ranki, rankj := rankOfKind(objs[i].Kind), rankOfKind(objs[j].Kind)
	if ranki == rankj {
		return objs[i].Metadata.Name < objs[j].Metadata.Name
	}
	return ranki < rankj
}

// rankOfKind returns an int denoting the position of the given kind
// in the partial ordering of Kubernetes resources, according to which
// kinds depend on which (derived by hand).
func rankOfKind(kind string) int {
	switch kind {
	// Namespaces answer to NOONE
	case "Namespace":
		return 0
		// These don't go in namespaces; or do, but don't depend on anything else
	case "ServiceAccount", "ClusterRole", "Role", "PersistentVolume", "Service":
		return 1
		// These depend on something above, but not each other
	case "ResourceQuota", "LimitRange", "Secret", "ConfigMap", "RoleBinding", "ClusterRoleBinding", "PersistentVolumeClaim", "Ingress":
		return 2
		// Same deal, next layer
	case "DaemonSet", "Deployment", "ReplicationController", "ReplicaSet", "Job", "CronJob", "StatefulSet":
		return 3
		// Assumption: anything not mentioned isn't depended _upon_, so
		// can come last.
	default:
		return 4
	}
}

func makeMultidoc(objs []*kubernetes.ApiObject) *bytes.Buffer {
	buf := &bytes.Buffer{}
	for _, obj := range objs {
		buf.WriteString("\n---\n")
		buf.Write(obj.Bytes())
	}
	return buf
}
