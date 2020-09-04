package app

import (
	"github.com/kubeedge/beehive/pkg/core"
	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/kubernetes/cmd/cloudgateway/app/options"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudhub"
	"k8s.io/kubernetes/pkg/cloudgateway/controller"
	utilflag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/version/verflag"
)

func NewCloudGatewayCommand() *cobra.Command {
	o := options.NewOptions()
	cmd := &cobra.Command{
		Use: "cloudgateway",
		Long: `As the proxy or gateway of the services or component in the edge site, cloudgateway provides secure
communication and access capabilities for services and components of the cloud and edge sites.`,
		RunE: func(cmd *cobra.Command, args []string) error{
			utilflag.PrintFlags(cmd.Flags())

			config, err := o.Config()
			if err != nil {
				return err
			}

			// validate options
			if errs := options.Validate(config); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			// register all the modules started in cloudGateway
			registerModules(config)

			// start all the modules started in cloudGateway
			core.Run()

			return nil
		},
	}

	fs := cmd.Flags()
	namedFlagSets := o.Flags()
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name())
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	return cmd
}

// registerModules register all the modules started in cloudGateway
func registerModules(c *v1.CloudGatewayConfig) {
	cloudhub.Register(c.Modules.CloudHub)
	controller.Register(c.Modules.Controller, c.KubeAPIConfig)
}
