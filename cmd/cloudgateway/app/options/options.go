package options

import (
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/cloudgateway"
	"path"
	"sigs.k8s.io/yaml"

	cliflag "k8s.io/component-base/cli/flag"
	v1 "k8s.io/kubernetes/pkg/apis/cloudgateway/v1"
	"k8s.io/kubernetes/pkg/cloudgateway/common/constants"
)

// Options runs a cloudgateway
type Options struct {
	ConfigFile string
}

// NewOptions creates a new Options object with default parameters
func NewOptions() *Options {
	o := Options{
		ConfigFile: path.Join(constants.DefaultConfigDir, constants.DefaultConfigFile),
	}

	return &o
}

// Flags returns flags for a specific CloudGateway by section name
func (o *Options) Flags() (fss cliflag.NamedFlagSets) {
	fs := fss.FlagSet("cloudgateway")
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "Path to the CloudGateway configuration file."+
		" Flags override values in this file.")

	return fss
}

func (o *Options) Config() (*v1.CloudGatewayConfig, error) {
	cfg := NewCloudGatewayConfig()
	data, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		klog.Errorf("Failed to read config file %s: %v", o.ConfigFile, err)
		return nil, err
	}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		klog.Errorf("Failed to unmarshal config file %s: %v", o.ConfigFile, err)
		return nil, err
	}
	return cfg, nil
}

// NewCloudGatewayConfig returns a full CloudGatewayConfig object
func NewCloudGatewayConfig() *v1.CloudGatewayConfig {
	advertiseAddress, _ := utilnet.ChooseHostInterface()

	c := &v1.CloudGatewayConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       cloudgateway.Kind,
			APIVersion: path.Join(cloudgateway.GroupName, cloudgateway.Version),
		},
		KubeAPIConfig: &v1.KubeAPIConfig{
			Master:		"",
			KubeConfig:	constants.DefaultKubeConfig,
		},
		Modules: &v1.Modules{
			CloudHub: &v1.CloudHub{
				Enable:                  true,
				KeepaliveInterval:       30,
				NodeLimit:               1000,
				TLSCAFile:               constants.DefaultCAFile,
				TLSCAKeyFile:            constants.DefaultCAKeyFile,
				TLSCertFile:             constants.DefaultCertFile,
				TLSPrivateKeyFile:       constants.DefaultKeyFile,
				WriteTimeout:            30,
				AdvertiseAddress:        []string{advertiseAddress.String()},
				EdgeCertSigningDuration: 365,
				Quic: &v1.CloudHubQUIC{
					Enable:             false,
					Address:            "0.0.0.0",
					Port:               10001,
					MaxIncomingStreams: 10000,
				},
				WebSocket: &v1.CloudHubWebSocket{
					Enable:  true,
					Port:    10000,
					Address: "0.0.0.0",
				},
				HTTPS: &v1.CloudHubHTTPS{
					Enable:  true,
					Port:    10002,
					Address: "0.0.0.0",
				},
			},
		},
	}
	return c
}
