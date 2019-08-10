package main

import (
	"flag"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/workload-controller-manager/app"
	"k8s.io/kubernetes/pkg/cloudfabric-controller/api/types/v1alpha1"
	clientV1alpha1 "k8s.io/kubernetes/pkg/cloudfabric-controller/clientset/v1alpha1"
	"os"

	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	cloudFabricControllerConfig "k8s.io/kubernetes/cmd/workload-controller-manager/app/config"
)

var kubeconfig string

const (
	// WorkloadControllerManagerUserAgent is the userAgent name when starting cloudfabric-controller managers.
	WorkloadControllerManagerUserAgent = "workload-controller-manager"
)

func init() {
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfigEnv, "absolute path to the kubeconfig file")
	flag.Parse()
}

func main() {
	var config *rest.Config
	var err error

	logs.InitLogs()
	defer logs.FlushLogs()

	if kubeconfig == "" {
		fmt.Println("using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		fmt.Println("using configuration from ", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		panic(err)
	}

	v1alpha1.AddToScheme(scheme.Scheme)

	clientSet, err := clientV1alpha1.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	controllermanagers, err := clientSet.ControllerManagers("default").List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("controllermanagers found: %+v\n", controllermanagers)

	/*
		store := WatchResources(clientSet)

		for {
			controllermanagersFromStore := store.List()
			fmt.Printf("controllermanagers in store: %d\n", len(controllermanagersFromStore))

			time.Sleep(2 * time.Second)
		}*/

	cloudFabricConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	client, err := clientset.NewForConfig(restclient.AddUserAgent(cloudFabricConfig, WorkloadControllerManagerUserAgent))
	if err != nil {
		panic(err)
	}

	//eventRecorder := createRecorder(client, WorkloadControllerManagerUserAgent)

	c := &cloudFabricControllerConfig.Config{
		Client:            client,
		CloudFabricConfig: cloudFabricConfig,
		//EventRecorder:        eventRecorder,
	}

	app.StartControllerManager(c.Complete(), wait.NeverStop)
}

/*
func createRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, v1.EventSource{Component: userAgent})
}
*/
