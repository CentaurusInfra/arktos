/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

@CHANGELOG
KubeEdge Authors: To create mini-kubelet for edge deployment scenario,
This file is derived from K8S Kubelet code with reduced set of methods
Changes done are
1. setEdgeClusterReadyCondition is partially come from "k8s.io/kubernetes/pkg/kubelet.setEdgeClusterReadyCondition"
*/

package edged

import (
	"fmt"
	"os"
	"path/filepath"

	edgeclustersv1 "github.com/kubeedge/kubeedge/cloud/pkg/apis/edgeclusters/v1"
	"k8s.io/klog/v2"

	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/kubeedge/edge/pkg/common/message"
	"github.com/kubeedge/kubeedge/edge/pkg/common/modules"
	"github.com/kubeedge/kubeedge/edge/pkg/edged/config"
	"github.com/kubeedge/kubeedge/edge/pkg/edgehub"
	"github.com/kubeedge/kubeedge/edge/pkg/edged/missionmanager"
	edgedconfig "github.com/kubeedge/kubeedge/edge/pkg/edged/config"
	v1 "k8s.io/api/core/v1"
)

var initEdgeCluster edgeclustersv1.EdgeCluster


func (e *edged) initialEdgeCluster() (*edgeclustersv1.EdgeCluster, error) {
	var edgeCluster = &edgeclustersv1.EdgeCluster{}

	if err := checkEdgeClusterConfig(); err != nil {
		return nil, err
	}

	edgeClusterConfig := edgedconfig.Config.EdgeCluster

	edgeCluster.Name = edgeClusterConfig.Name
	edgeCluster.Spec.ClusterKubeconfig = edgeClusterConfig.ClusterKubeconfig
	edgeCluster.Spec.KubeDistro = edgeClusterConfig.KubeDistro	

	edgeCluster.Labels = map[string]string{
		// Kubernetes built-in labels
		v1.LabelHostname:   edgeCluster.Name,

		// KubeEdge specific labels
		"edgeCluster-role.kubernetes.io/edge-cluster":  "",
	}

	for k, v := range edgeClusterConfig.Labels {
		edgeCluster.Labels[k] = v
	}

	// The status 
	edgeCluster.Status.Ready = true

	return edgeCluster, nil
}

func checkEdgeClusterConfig() error {
	edgeClusterConfig := edgedconfig.Config.EdgeCluster

	if !missionmanager.FileExists(edgeClusterConfig.ClusterKubeconfig) {
		return fmt.Errorf("Could not open kubeconfig file (%s)", edgeClusterConfig.ClusterKubeconfig)
	}

	if _, exists := missionmanager.DistroToKubectl[edgeClusterConfig.KubeDistro]; !exists {
		return fmt.Errorf("Invalid kube distribution (%v)", edgeClusterConfig.KubeDistro)
	}

	clusterName := edgeClusterConfig.Name
	if len(clusterName) == 0 {
		var err error
		edgedconfig.Config.EdgeCluster.Name, err = os.Hostname()
		if err != nil {
			klog.Errorf("The cluster name is empty, and couldn't determine hostname: %v", err)
			return  err
		}
	}

	basedir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	kubectlPath := filepath.Join(basedir, missionmanager.DistroToKubectl[edgeClusterConfig.KubeDistro])

	test_cluster_command := fmt.Sprintf("%s cluster-info --kubeconfig=%s", kubectlPath, edgeClusterConfig.ClusterKubeconfig)
	exitcode, output, err := missionmanager.ExecCommandLine(test_cluster_command, missionmanager.COMMAND_TIMEOUT_SEC)
	if exitcode != 0 || err != nil {
		return fmt.Errorf("The cluster is unreachable. Command (%s) failed: exit code: v, output: %v, err: %v", exitcode, output, err)
	}

	return nil
}

func (e *edged) setInitEdgeCluster(edgeCluster *edgeclustersv1.EdgeCluster) {
	initEdgeCluster.Status = *edgeCluster.Status.DeepCopy()
}

func (e *edged) registerEdgeCluster() error {
	edgeCluster, err := e.initialEdgeCluster()
	if err != nil {
		klog.Errorf("Unable to construct edgeclustersv1.EdgeCluster object for edge: %v", err)
		return err
	}

	e.setInitEdgeCluster(edgeCluster)

	if !config.Config.RegisterNode {
		//when register-edgeCluster set to false, do not auto register edgeCluster
		klog.Infof("register-edgeCluster is set to false")
		e.registrationCompleted = true
		return nil
	}

	klog.Infof("Attempting to register edgeCluster %s", edgeCluster.Name)

	resource := fmt.Sprintf("%s/%s/%s", e.namespace, model.ResourceTypeEdgeClusterStatus, edgeCluster.Name)
	edgeClusterInfoMsg := message.BuildMsg(modules.MetaGroup, "", modules.EdgedModuleName, resource, model.InsertOperation, edgeCluster)
	var res model.Message
	if _, ok := core.GetModules()[edgehub.ModuleNameEdgeHub]; ok {
		res, err = beehiveContext.SendSync(edgehub.ModuleNameEdgeHub, *edgeClusterInfoMsg, syncMsgRespTimeout)
	} else {
		res, err = beehiveContext.SendSync(EdgeController, *edgeClusterInfoMsg, syncMsgRespTimeout)
	}

	if err != nil || res.Content != "OK" {
		klog.Errorf("register edgeCluster failed, error: %v", err)
		if res.Content != "OK" {
			klog.Errorf("response from cloud core: %v", res.Content)
		}
		return err
	}

	klog.Infof("Successfully registered edgeCluster %s", edgeCluster.Name)
	e.registrationCompleted = true

	return nil
}

func (e *edged) updateEdgeClusterStatus() error {
	/*edgeClusterStatus, err := e.getEdgeClusterStatusRequest(&initEdgeCluster)
	if err != nil {
		klog.Errorf("Unable to construct api.EdgeClusterStatusRequest object for edge: %v", err)
		return err
	}

	err = e.metaClient.EdgeClusterStatus(e.namespace).Update(edgeCluster.Name, *edgeClusterStatus)
	if err != nil {
		klog.Errorf("update edgeCluster failed, error: %v", err)
	}*/
	return nil
}

func (e *edged) syncEdgeClusterStatus() {
	if !e.registrationCompleted {
		if err := e.registerEdgeCluster(); err != nil {
			klog.Errorf("Register edgeCluster failed: %v", err)
			return
		}
	}

	if err := e.updateEdgeClusterStatus(); err != nil {
		klog.Errorf("Unable to update edgeCluster status: %v", err)
	}
}
