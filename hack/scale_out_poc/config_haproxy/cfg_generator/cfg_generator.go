/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this tsFile except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"k8s.io/klog"
)

var (
	templateFile        string
	cfgGenerated        string
	tenantPartititonIPs []string
	resourcePartitionIP string
)

const (
	ARKTOS_API_PORT      = 8080
	MAX_CONN_PER_BACKEND = 500000
	CONNECTION_TIMEOUT   = "10m"
	PROXY_PORT           = "8888"
)

func initFlags() {
	flag.StringVar(&templateFile, "template", "", "The path to the source template cfg file")
	flag.StringVar(&cfgGenerated, "target", "", "The path to the generated haproxy cfg file")

	flag.Parse()

	validateFlags()
}

func validateFlags() {
	if strings.TrimSpace(cfgGenerated) == "" {
		LogError("\nNo target file is specified.\n")
		os.Exit(1)
	}

	if !FileExists(templateFile) {
		LogError("\nTemplate file (%v) does not exist.\n", templateFile)
		os.Exit(1)
	}
}

func getPartitionIPs() {
	TP_IP_str := os.Getenv("TENANT_PARTITION_IP")
	if len(TP_IP_str) == 0 {
		LogError("\nTENANT_PARTITION_IP is not specified.\n")
		os.Exit(1)
	}

	for _, ip := range strings.Split(TP_IP_str, ",") {
		ip_addr := strings.TrimSpace(ip)
		if net.ParseIP(ip_addr) == nil {
			LogError("\nIP address (%v) is invalid.\n", ip_addr)
			os.Exit(1)
		}
		tenantPartititonIPs = append(tenantPartititonIPs, ip_addr)
	}

	if len(tenantPartititonIPs) > 26 {
		LogError("\nUnable to support TP # > 26.\n")
		os.Exit(1)
	}

	resourcePartitionIP = strings.TrimSpace(os.Getenv("RESOURCE_PARTITION_IP"))
	if len(resourcePartitionIP) == 0 {
		LogError("\nRESOURCE_PARTITION_IP is not specified.\n")
		os.Exit(1)
	}

	if net.ParseIP(resourcePartitionIP) == nil {
		LogError("\nIP address (%v) is invalid.\n", resourcePartitionIP)
		os.Exit(1)
	}
}

func get_tp_request_acl() string {

	result := ""
	letters := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}

	tp_request_1_acl := "\n    acl tp_%v_request_2 path_reg ^/api/[a-z0-9_.-]+/tenants/(?!(system$|system/.*$|all$|all/.*$))([%v-%v].*)$"
	tp_request_2_acl := "\n    acl tp_%v_request_1 path_reg ^/apis/[a-z0-9_.-]+/[a-z0-9_.-]+/tenants/(?!(system$|system/.*$|all$|all/.*$))([%v-%v].*)$\n"

	width := 26.0 / float32(len(tenantPartititonIPs))
	for index := range tenantPartititonIPs {
		start_letter := letters[int(width*float32(index))]
		end_letter := letters[int(width*(float32(index)+1))-1]

		result = result + fmt.Sprintf(tp_request_1_acl, index+1, start_letter, end_letter)
		result = result + fmt.Sprintf(tp_request_2_acl, index+1, start_letter, end_letter)
	}
	return result
}

func get_rp_request_acl() string {
	return `acl node_request path_reg ^/api/[a-z0-9_.-]+/nodes.*$
    acl lease_request path_reg ^/apis/coordination.k8s.io/[a-z0-9_.-]+/leases.*$
    acl individual_lease_request path_reg ^/apis/[a-z0-9_.-]+/[a-z0-9_.-]+/tenants/system/namespaces/kube-node-lease/leases.*$
	`
}

func get_tp_source_acl() string {

	result := ""

	tp_source_acl := "\n    acl from_tenant_api_%v src %v"
	for index, ip := range tenantPartititonIPs {
		result = result + fmt.Sprintf(tp_source_acl, index+1, ip)
	}
	return result
}

func get_rp_source_acl() string {

	result := ""

	rp_source_acl := "\n    acl from_resource_api src %v"

	result = result + fmt.Sprintf(rp_source_acl, resourcePartitionIP)

	return result
}

func get_rp_request_rule() string {
	return "use_backend resource_api if node_request OR lease_request OR individual_lease_request"
}

func get_tp_request_rule() string {

	result := ""

	tp_request_rule := "\n    use_backend tenant_api_%v if tp_%v_request_1 OR tp_%v_request_2"
	for index := range tenantPartititonIPs {
		result = result + fmt.Sprintf(tp_request_rule, index+1, index+1, index+1)
	}
	return result
}

func get_partition_source_rule() string {
	result := ""

	tp_source_request_rule := "\n    use_backend tenant_api_%v if from_tenant_api_%v"
	for index := range tenantPartititonIPs {
		result = result + fmt.Sprintf(tp_source_request_rule, index+1, index+1)
	}

	rp_source_request_rule := "\n    use_backend resource_api if from_resource_api"
	result = result + rp_source_request_rule

	return result
}

func get_backends() string {

	result := ""

	rp_backend := "backend tenant_api_%v\n    server tp_%v %v:%v maxconn %v\n\n"
	for index, ip := range tenantPartititonIPs {
		result = result + fmt.Sprintf(rp_backend, index+1, index+1, ip, ARKTOS_API_PORT, MAX_CONN_PER_BACKEND)
	}

	tp_backend := "backend resource_api\n    server rp %v:%v maxconn %v\n\n"
	result = result + fmt.Sprintf(tp_backend, resourcePartitionIP, ARKTOS_API_PORT, MAX_CONN_PER_BACKEND)

	return result
}

func createCfg() {
	content, err := ioutil.ReadFile(templateFile)
	if err != nil {
		LogError("Error in loading template source file %v : %v ", templateFile, err)
		os.Exit(1)
	}
	contentString := string(content)

	contentString = strings.ReplaceAll(contentString, "{{ tp_request_acl }}", get_tp_request_acl())
	contentString = strings.ReplaceAll(contentString, "{{ rp_request_acl }}", get_rp_request_acl())
	contentString = strings.ReplaceAll(contentString, "{{ tp_source_acl }}", get_tp_source_acl())
	contentString = strings.ReplaceAll(contentString, "{{ rp_source_acl }}", get_rp_source_acl())

	contentString = strings.ReplaceAll(contentString, "{{ tp_request_rule }}", get_tp_request_rule())
	contentString = strings.ReplaceAll(contentString, "{{ rp_request_rule }}", get_rp_request_rule())
	contentString = strings.ReplaceAll(contentString, "{{ partition_source_rule }}", get_partition_source_rule())

	contentString = strings.ReplaceAll(contentString, "{{ backends }}", get_backends())
	contentString = strings.ReplaceAll(contentString, "{{ connection_timeout }}", CONNECTION_TIMEOUT)
	contentString = strings.ReplaceAll(contentString, "{{ proxy_port }}", PROXY_PORT)

	err = ioutil.WriteFile(cfgGenerated, []byte(contentString), 0644)
	if err != nil {
		LogError("Error in writing to file %v : %v ", cfgGenerated, err)
		os.Exit(1)
	}
}

func main() {
	defer klog.Flush()

	initFlags()

	getPartitionIPs()

	createCfg()

	LogSuccess("Success: %v generated. \n", cfgGenerated)
}
