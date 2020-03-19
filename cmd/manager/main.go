package main

import (
	goflag "flag"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/cmd/manager/options"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"

	"github.com/golang/glog"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func init() {
	goflag.Set("logtostderr", "true")
}

func main() {
	getter := genericclioptions.NewConfigFlags(true)
	command := options.NewAgentCommand(cmdutil.NewFactory(getter))

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	_ = goflag.CommandLine.Parse([]string{})

	defer glog.Flush()

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
