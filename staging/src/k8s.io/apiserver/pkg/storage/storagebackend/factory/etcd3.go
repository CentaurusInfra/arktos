/*
Copyright 2016 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package factory

import (
	"context"
	"fmt"
	"io/ioutil"
	"k8s.io/apiserver/pkg/storage/datapartition"
	"k8s.io/apiserver/pkg/storage/storagecluster"
	"k8s.io/klog"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/etcd3"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/value"
)

// The short keepalive timeout and interval have been chosen to aggressively
// detect a failed etcd server without introducing much overhead.
const keepaliveTime = 30 * time.Second
const keepaliveTimeout = 10 * time.Second

// dialTimeout is the timeout for failing to establish a connection.
// It is set to 20 seconds as times shorter than that will cause TLS connections to fail
// on heavily loaded arm64 CPUs (issue #64649)
const dialTimeout = 20 * time.Second

// TODO - health check for data clusters
func newETCD3HealthCheck(c storagebackend.Config) (func() error, error) {
	// constructing the etcd v3 client blocks and times out if etcd is not available.
	// retry in a loop in the background until we successfully create the client, storing the client or error encountered

	clientValue := &atomic.Value{}

	clientErrMsg := &atomic.Value{}
	clientErrMsg.Store("etcd client connection not yet established")

	go wait.PollUntil(time.Second, func() (bool, error) {
		client, err := newETCD3Client(c.Transport, c.Transport.SystemClusterServerList)
		if err != nil {
			clientErrMsg.Store(err.Error())
			return false, nil
		}
		clientValue.Store(client)
		clientErrMsg.Store("")
		return true, nil
	}, wait.NeverStop)

	return func() error {
		if errMsg := clientErrMsg.Load().(string); len(errMsg) > 0 {
			return fmt.Errorf(errMsg)
		}
		client := clientValue.Load().(*clientv3.Client)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// See https://github.com/etcd-io/etcd/blob/master/etcdctl/ctlv3/command/ep_command.go#L118
		_, err := client.Get(ctx, path.Join(c.Prefix, "health"))
		if err == nil {
			return nil
		}
		return fmt.Errorf("error getting data from etcd: %v", err)
	}, nil
}

func newETCD3Client(c storagebackend.TransportConfig, servers []string) (*clientv3.Client, error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      c.CertFile,
		KeyFile:       c.KeyFile,
		TrustedCAFile: c.TrustedCAFile,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}
	// NOTE: Client relies on nil tlsConfig
	// for non-secure connections, update the implicit variable
	if len(c.CertFile) == 0 && len(c.KeyFile) == 0 && len(c.TrustedCAFile) == 0 {
		tlsConfig = nil
	}
	cfg := clientv3.Config{
		DialTimeout:          dialTimeout,
		DialKeepAliveTime:    keepaliveTime,
		DialKeepAliveTimeout: keepaliveTimeout,
		DialOptions: []grpc.DialOption{
			grpc.WithUnaryInterceptor(grpcprom.UnaryClientInterceptor),
			grpc.WithStreamInterceptor(grpcprom.StreamClientInterceptor),
		},
		Endpoints: servers,
		TLS:       tlsConfig,
	}

	return clientv3.New(cfg)
}

type runningCompactor struct {
	interval time.Duration
	cancel   context.CancelFunc
	client   *clientv3.Client
	refs     int
}

var (
	lock       sync.Mutex
	compactors = map[string]*runningCompactor{}
)

// startCompactorOnce start one compactor per transport. If the interval get smaller on repeated calls, the
// compactor is replaced. A destroy func is returned. If all destroy funcs with the same transport are called,
// the compactor is stopped.
func startCompactorOnce(c storagebackend.TransportConfig, servers []string, interval time.Duration) (func(), error) {
	lock.Lock()
	defer lock.Unlock()

	key := fmt.Sprintf("%v", c) // gives: {[server1 server2] keyFile certFile caFile}
	if compactor, foundBefore := compactors[key]; !foundBefore || compactor.interval > interval {
		compactorClient, err := newETCD3Client(c, servers)
		if err != nil {
			return nil, err
		}

		if foundBefore {
			// replace compactor
			compactor.cancel()
			compactor.client.Close()
		} else {
			// start new compactor
			compactor = &runningCompactor{}
			compactors[key] = compactor
		}

		ctx, cancel := context.WithCancel(context.Background())

		compactor.interval = interval
		compactor.cancel = cancel
		compactor.client = compactorClient

		etcd3.StartCompactor(ctx, compactorClient, interval)
	}

	compactors[key].refs++

	return func() {
		lock.Lock()
		defer lock.Unlock()

		compactor := compactors[key]
		compactor.refs--
		if compactor.refs == 0 {
			compactor.cancel()
			compactor.client.Close()
			delete(compactors, key)
		}
	}, nil
}

// TODO - start/stop compactor for data clusters
func newETCD3Storage(c storagebackend.Config) (storage.Interface, DestroyFunc, error) {
	stopCompactor, err := startCompactorOnce(c.Transport, c.Transport.SystemClusterServerList, c.CompactionInterval)
	if err != nil {
		return nil, nil, err
	}

	client, err := newETCD3Client(c.Transport, c.Transport.SystemClusterServerList)
	if err != nil {
		stopCompactor()
		return nil, nil, err
	}

	var once sync.Once
	destroyFunc := func() {
		// we know that storage destroy funcs are called multiple times (due to reuse in subresources).
		// Hence, we only destroy once.
		// TODO: fix duplicated storage destroy calls higher level
		once.Do(func() {
			stopCompactor()
			client.Close()
		})
	}
	transformer := c.Transformer
	if transformer == nil {
		transformer = value.IdentityTransformer
	}

	var store storage.StorageClusterInterface
	updatePartitionChGrp := datapartition.GetDataPartitionUpdateChGrp()
	updatePartitionCh := updatePartitionChGrp.Join()
	if c.PartitionConfigFilepath != "" {
		configMap, _ := parseConfig(c.PartitionConfigFilepath)
		store = etcd3.NewWithPartitionConfig(client, c.Codec, c.Prefix, transformer, c.Paging, configMap, updatePartitionCh)
	} else {
		store = etcd3.New(client, c.Codec, c.Prefix, transformer, c.Paging, updatePartitionCh)
	}

	updateStorageClusterCh := storagecluster.GetStorageClusterUpdateCh()
	go func(s storage.StorageClusterInterface, transport storagebackend.TransportConfig, updateClusterCh chan storagecluster.StorageClusterAction) {
		for storageClusterAction := range updateStorageClusterCh {
			switch storageClusterAction.Action {
			case "ADD":
				newDataClient, err := newETCD3Client(transport, storageClusterAction.ServerAddresses)
				if err != nil {
					// TODO - start compact for new data cluster
				}

				err = s.AddDataClient(newDataClient, storageClusterAction.StorageClusterId)
				if err != nil {
					klog.Fatalf("Unexpected storage cluster add event. Error %v", err)
				}
			case "UPDATE":
				newDataClient, err := newETCD3Client(transport, storageClusterAction.ServerAddresses)
				if err != nil {
					// TODO - start compact for new data cluster
				}

				err = s.UpdateDataClient(newDataClient, storageClusterAction.StorageClusterId)
				if err != nil {
					klog.Fatalf("Unexpected storage cluster update event. Error %v", err)
				}
				// TODO - stop compact for original cluster

			case "DELETE":
				s.DeleteDataClient(storageClusterAction.StorageClusterId)

				// TODO - stop compact for original data cluster
			default:
				klog.Errorf("Got invalid storage cluster action [%+v]", storageClusterAction)
			}
		}
	}(store, c.Transport, updateStorageClusterCh)

	return store, destroyFunc, nil
}

// Each line in the config needs to contain three part: keyName, start, end
// End is excludsive in the range, for example, below line means partition pods, the keys in the partition are ns1 and ns2
// /registry/pods/, ns1, ns3
// /registry/minions/, node1, node100
func parseConfig(configFileName string) (map[string]storage.Interval, error) {
	m := make(map[string]storage.Interval)

	bytes, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return m, err
	}
	lines := strings.Split(string(bytes), "\n")

	for _, line := range lines {
		slices := strings.Split(line, ",")
		size := len(slices)
		if size >= 3 {
			m[strings.TrimSpace(slices[0])] = storage.Interval{Begin: strings.TrimSpace(slices[1]), End: strings.TrimSpace(slices[2])}
		}
	}
	return m, nil
}
