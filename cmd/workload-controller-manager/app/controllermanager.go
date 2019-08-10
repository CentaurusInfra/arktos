package app

import (
	"context"
	"fmt"
	"k8s.io/klog"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubernetes/pkg/cloudfabric-controller"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/replicaset"

	"k8s.io/kubernetes/cmd/workload-controller-manager/app/config"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	componentbaseconfig "k8s.io/component-base/config"
)

// ControllerContext defines the context object for controller
type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	// GenericInformerFactory gives access to informers for typed resources
	// and dynamic resources.
	GenericInformerFactory controller.InformerFactory

	// DeferredDiscoveryRESTMapper is a RESTMapper that will defer
	// initialization of the RESTMapper until the first mapping is
	// requested.
	RESTMapper *restmapper.DeferredDiscoveryRESTMapper

	// AvailableResources is a map listing currently available resources
	AvailableResources map[schema.GroupVersionResource]bool

	// Stop is the stop channel
	Stop <-chan struct{}

	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}
}

var resyncPeriod = time.Duration(60 * time.Second)

func StartControllerManager(c *config.CompletedConfig, stopCh <-chan struct{}) error {
	// Setup any healthz checks we will want to use.
	var checks []healthz.HealthzChecker

	// Start the controller manager HTTP server
	// unsecuredMux is the handler for these controller *after* authn/authz filters have been applied
	var unsecuredMux *mux.PathRecorderMux
	debugConfig := &componentbaseconfig.DebuggingConfiguration{}
	if c.SecureServing != nil {
		unsecuredMux = NewBaseHandler(debugConfig, checks...)
		handler := BuildHandlerChain(unsecuredMux, &c.Authorization, &c.Authentication)
		// TODO: handle stoppedCh returned by c.SecureServing.Serve
		if _, err := c.SecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}
	if c.InsecureServing != nil {
		unsecuredMux = NewBaseHandler(debugConfig, checks...)
		insecureSuperuserAuthn := server.AuthenticationInfo{Authenticator: &server.InsecureSuperuser{}}
		handler := BuildHandlerChain(unsecuredMux, nil, &insecureSuperuserAuthn)
		if err := c.InsecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}

	rootClientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: c.CloudFabricConfig,
	}

	clientBuilder := rootClientBuilder

	ctx := context.TODO()

	controllerContext, err := CreateControllerContext(rootClientBuilder, clientBuilder, ctx.Done())

	if err != nil {
		klog.Fatalf("error building controller context: %v", err)
	}

	startReplicaSetController(controllerContext)

	controllerContext.InformerFactory.Start(controllerContext.Stop)
	controllerContext.GenericInformerFactory.Start(controllerContext.Stop)
	close(controllerContext.InformersStarted)

	select {}
	panic("unreachable")
}

//func CreateControllerContext(s *config.CompletedConfig, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (ControllerContext, error) {
func CreateControllerContext(rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (ControllerContext, error) {
	versionedClient := rootClientBuilder.ClientOrDie("shared-informers")
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, resyncPeriod)

	dynamicClient := dynamic.NewForConfigOrDie(rootClientBuilder.ConfigOrDie("dynamic-informers"))
	dynamicInformers := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, resyncPeriod)

	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	if err := WaitForAPIServer(versionedClient, 10*time.Second); err != nil {
		return ControllerContext{}, fmt.Errorf("failed to wait for apiserver being healthy: %v", err)
	}

	// Use a discovery client capable of being refreshed.
	discoveryClient := rootClientBuilder.ClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, stop)

	availableResources, err := GetAvailableResources(rootClientBuilder)
	if err != nil {
		return ControllerContext{}, err
	}

	/*cloud, loopMode, err := createCloudProvider(s.ComponentConfig.KubeCloudShared.CloudProvider.Name, s.ComponentConfig.KubeCloudShared.ExternalCloudVolumePlugin,
		s.ComponentConfig.KubeCloudShared.CloudProvider.CloudConfigFile, s.ComponentConfig.KubeCloudShared.AllowUntaggedCloud, sharedInformers)
	if err != nil {
		return ControllerContext{}, err
	}
	*/

	ctx := ControllerContext{
		ClientBuilder:          clientBuilder,
		InformerFactory:        sharedInformers,
		GenericInformerFactory: controller.NewInformerFactory(sharedInformers, dynamicInformers),
		RESTMapper:             restMapper,
		AvailableResources:     availableResources,
		Stop:                   stop,
		InformersStarted:       make(chan struct{}),
	}
	return ctx, nil
}

// GetAvailableResources gets the map which contains all available resources of the apiserver
// TODO: In general, any controller checking this needs to be dynamic so
// users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func GetAvailableResources(clientBuilder controller.ControllerClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	client := clientBuilder.ClientOrDie("controller-discovery")
	discoveryClient := client.Discovery()
	resourceMap, err := discoveryClient.ServerResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resourceMap) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

func startReplicaSetController(ctx ControllerContext) (http.Handler, bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}] {
		return nil, false, nil
	}
	go replicaset.NewReplicaSetController(
		ctx.InformerFactory.Apps().V1().ReplicaSets(),
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie("replicaset-controller"),
		replicaset.BurstReplicas,
	).Run(ctx.Stop)
	return nil, true, nil
}
