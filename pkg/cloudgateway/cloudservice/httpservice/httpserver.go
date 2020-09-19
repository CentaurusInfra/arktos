package httpservice

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	beehiveModel "github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudgateway/cloudservice/utils"
	"k8s.io/kubernetes/pkg/cloudgateway/common/modules"
)

const (
	address = "127.0.0.1"
	port    = 1234
)

func StartHttpServer() {
	router := mux.NewRouter()
	router.HandleFunc("/", RequestFunc)
	addr := fmt.Sprintf("%s:%d", address, port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	klog.Fatal(server.ListenAndServe())
}

func RequestFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	var httpRequest HTTPRequest
	httpRequest.Header = r.Header
	httpRequest.Body = data
	message := beehiveModel.NewMessage("")
	message.Content = httpRequest
	host := strings.Split(r.Host, ":")
	resource, err := utils.GenerateResource(host[0])
	if err != nil {
		klog.Errorf("target service cannot reach: %v", err)
		s := "target service cannot reach: " + err.Error() + "\n"
		w.Write([]byte(s))
		return
	}
	message.BuildRouter(modules.CloudServiceModuleName, modules.EdgeServiceGroup, resource, r.Method)

	// send message to cloudhub
	respMessage, err := beehiveContext.SendSync(modules.HubGroup, *message, 30*time.Second)
	if err != nil {
		klog.Errorf("failed to send message to cloudhub: %v", err)
		s := "failed to send message: " + err.Error() + "\n"
		w.Write([]byte(s))
		return
	}
	// Marshal response message
	resp, err := json.Marshal(respMessage.GetContent())
	if err != nil {
		klog.Errorf("marshal response failed with error: %v", err)
		s := "failed to marshal response message: " + err.Error() + "\n"
		w.Write([]byte(s))
		return
	}
	var httpResponse HTTPResponse
	if err = json.Unmarshal(resp, &httpResponse); err != nil {
		klog.Errorf("error to parse http: %v", err)
		s := "error to parse http response: " + err.Error() + "\n"
		w.Write([]byte(s))
		return
	}

	// return response of the request
	w.Write(httpResponse.Body)
}
