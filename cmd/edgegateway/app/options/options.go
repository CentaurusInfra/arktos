package options

import (
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/edgegateway"
	v1 "k8s.io/kubernetes/pkg/apis/edgegateway/v1"
	"k8s.io/kubernetes/pkg/edgegateway/common/constants"
)

// Options runs a EdgeGateway
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

// Flags returns flags for a specific EdgeGateway by section name
func (o *Options) Flags() (fss cliflag.NamedFlagSets) {
	fs := fss.FlagSet("edgegateway")
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "Path to the EdgeGateway configuration file."+
		" Flags override values in this file.")

	return fss
}

func (o *Options) Config() (*v1.EdgeGatewayConfig, error) {
	cfg := NewEdgeGatewayConfig()
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

// NewEdgeGatewayConfig returns a full EdgeGatewayConfig object
func NewEdgeGatewayConfig() *v1.EdgeGatewayConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = constants.DefaultHostnameOverride
	}
	localIP := constants.LocalIP

	return &v1.EdgeGatewayConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       edgegateway.Kind,
			APIVersion: path.Join(edgegateway.GroupName, edgegateway.Version),
		},
		Modules: &v1.Modules{
			EdgeHub: &v1.EdgeHub{
				Enable:            true,
				Heartbeat:         15,
				ProjectID:         "e632aba927ea4ac2b575ec1603d56f10",
				TLSCAFile:         constants.DefaultCAFile,
				TLSCertFile:       constants.DefaultCertFile,
				TLSPrivateKeyFile: constants.DefaultKeyFile,
				Hostname:          hostname,
				Quic: &v1.EdgeHubQUIC{
					Enable:           false,
					HandshakeTimeout: 30,
					ReadDeadline:     15,
					Server:           net.JoinHostPort(localIP, "10001"),
					WriteDeadline:    15,
				},
				WebSocket: &v1.EdgeHubWebSocket{
					Enable:           true,
					HandshakeTimeout: 30,
					ReadDeadline:     15,
					Server:           net.JoinHostPort(localIP, "10000"),
					WriteDeadline:    15,
				},
				HTTPServer: (&url.URL{
					Scheme: "https",
					Host:   net.JoinHostPort(localIP, "10002"),
				}).String(),
			},
		},
	}
}
