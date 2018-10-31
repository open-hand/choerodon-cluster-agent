package cmd

import (
	"github.com/choerodon/choerodon-cluster-agent/agent"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	_ "net/http/pprof"
)



func NewAgentCommand(f cmdutil.Factory) *cobra.Command {
	options := agent.NewAgentOptions()
	cmd := &cobra.Command{
		Use:  "choerodon-cluster-agent",
		Long: `Environment Agent`,
		Run: func(cmd *cobra.Command, args []string) {
			agent.Run(options, f)
		},
	}
	options.BindFlags(cmd.Flags())
	f.BindFlags(cmd.PersistentFlags())
	f.BindExternalFlags(cmd.PersistentFlags())

	return cmd
}








