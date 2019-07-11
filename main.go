package main

import (
	goflag "flag"
	"fmt"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/choerodon/choerodon-cluster-agent/cmd"
)

func init() {
	goflag.Set("logtostderr", "true")
}

func main() {
	getter := genericclioptions.NewConfigFlags()
	command := cmd.NewAgentCommand(cmdutil.NewFactory(getter))

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	goflag.CommandLine.Parse([]string{})

	defer glog.Flush()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
