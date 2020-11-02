package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/common/constants"
	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
)

type ServiceClient struct {
	Vip        string
	Client     map[string]string
	EdgeTapIP  string
	CloudTapIP string
}

type ServiceServer struct {
	Ip         string
	Vip        string
	ClientVip  []string
	EdgeTapIP  string
	CloudTapIP string
}

var (
	iptables utiliptables.Interface
	route    netlink.Route
)

func Init() {
	protocol := utiliptables.ProtocolIpv4
	exec := utilexec.New()
	dbus := utildbus.New()
	iptables = utiliptables.New(exec, dbus, protocol)
}

func MeshHandler(message model.Message) {
	// get service information from message
	content, err := json.Marshal(message.GetContent())
	if err != nil {
		klog.Errorf("marshall message content failed, error: %v", err)
		return
	}
	resource := strings.Split(message.GetResource(), "/")
	operation := message.GetOperation()
	// service is on the cloud
	if resource[3] == constants.ServiceServer {
		var serviceServer ServiceServer
		if err := json.Unmarshal(content, &serviceServer); err != nil {
			klog.Errorf("error to parse service server struct: %v", err)
			return
		}
		// set route and iptables
		for _, vip := range serviceServer.ClientVip {
			routeManager(vip, serviceServer.EdgeTapIP, operation)
			dNatRule := "-p tcp -s " + vip + " -d " + serviceServer.Vip + " -j DNAT --to-destination " + serviceServer.Ip
			ruleManager(utiliptables.ChainPrerouting, dNatRule, operation)
		}

		return
	}
	// the service is at the edge site
	var serviceClient ServiceClient
	if err := json.Unmarshal(content, &serviceClient); err != nil {
		klog.Errorf("error to parse service client struct: %v", err)
		return
	}
	// set iptables
	for ip, vip := range serviceClient.Client {
		sNatRule := "-p tcp -s " + ip + " -d " + serviceClient.Vip + " -j SNAT --to-source " + vip
		ruleManager(utiliptables.ChainPostrouting, sNatRule, operation)
	}
	// set route
	routeManager(serviceClient.Vip, serviceClient.EdgeTapIP, operation)
}

func routeManager(ip string, tapIP string, operation string) {
	dst, err := netlink.ParseIPNet(fmt.Sprintf("%s/%d", ip, 32))
	if err != nil {
		klog.Errorf("parse ip error: %v", err)
		return
	}
	gw := net.ParseIP(tapIP)
	route = netlink.Route{
		Dst: dst,
		Gw:  gw,
	}
	if operation == constants.Insert {
		err = netlink.RouteAdd(&route)
		if err != nil {
			klog.Errorf("failed to add a route, error: %v", err)
		}
	} else if operation == constants.Delete {
		err = netlink.RouteDel(&route)
		if err != nil {
			klog.Errorf("failed to delete a route, error: %v", err)
		}
	}
}

func ruleManager(chain utiliptables.Chain, natRule string, operation string) {
	rule := strings.Split(natRule, " ")
	if operation == constants.Insert {
		exist, err := iptables.EnsureRule(utiliptables.Append, utiliptables.TableNAT, chain, rule...)
		if err != nil {
			klog.Errorf("failed to ensure iptables rule, error: %v", err)
		}
		if !exist {
			klog.Infof("iptables rule %s not exists and created now", natRule)
		}
	} else if operation == constants.Delete {
		err := iptables.DeleteRule(utiliptables.TableNAT, chain, rule...)
		if err != nil {
			klog.Errorf("failed to delete iptables rule: %s, error: %v", natRule, err)
		}
	}
}
