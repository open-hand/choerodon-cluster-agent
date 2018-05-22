package main

import (
	goflag "flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/choerodon/choerodon-agent/cmd"
)

func init() {
	goflag.Set("logtostderr", "true")
}

func main() {
	command := cmd.NewAgentCommand(cmdutil.NewFactory(nil))

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	goflag.CommandLine.Parse([]string{})
	//pflag.Parse()

	defer glog.Flush()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
