package edgeservice

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/edgegateway/common/constants"
	"k8s.io/kubernetes/pkg/edgegateway/common/modules"
	"k8s.io/kubernetes/pkg/edgegateway/edgeservice/httpservice"
)

func messageRouter(message model.Message) {
	resource := message.GetResource()
	r := strings.Split(resource, ":")
	if len(r) != 2 {
		m := "the format of resource " + resource + " is incorrect"
		klog.Warningf(m)
		code := http.StatusBadRequest
		if response, err := buildErrorResponse(message.GetID(), m, code); err == nil {
			beehiveContext.SendToGroup(modules.EdgeHubGroup, response)
		}
		return
	}
	content, err := json.Marshal(message.GetContent())
	if err != nil {
		klog.Errorf("marshall message content failed %v", err)
		m := "error to marshal request msg content"
		code := http.StatusBadRequest
		if response, err := buildErrorResponse(message.GetID(), m, code); err == nil {
			beehiveContext.SendToGroup(modules.EdgeHubGroup, response)
		}
		return
	}
	var httpRequest httpservice.HTTPRequest
	if err := json.Unmarshal(content, &httpRequest); err != nil {
		m := "error to parse http request"
		code := http.StatusBadRequest
		klog.Errorf(m, err)
		if response, err := buildErrorResponse(message.GetID(), m, code); err == nil {
			beehiveContext.SendToGroup(modules.EdgeHubGroup, response)
		}
		return
	}

	operation := message.GetOperation()
	targetURL := "http://" + resource
	resp, err := httpservice.SendWithHTTP(operation, targetURL, httpRequest.Body)
	if err != nil {
		m := "error to call service"
		code := http.StatusNotFound
		klog.Errorf(m, err)
		if response, err := buildErrorResponse(message.GetID(), m, code); err == nil {
			beehiveContext.SendToGroup(modules.EdgeHubGroup, response)
		}
		return
	}

	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		m := "error to receive response, err: " + err.Error()
		code := http.StatusInternalServerError
		klog.Errorf(m, err)
		if response, err := buildErrorResponse(message.GetID(), m, code); err == nil {
			beehiveContext.SendToGroup(modules.EdgeHubGroup, response)
		}
		return
	}

	response := httpservice.HTTPResponse{Header: resp.Header, StatusCode: resp.StatusCode, Body: resBody}
	responseMsg := model.NewMessage(message.GetID())
	responseMsg.Content = response
	responseMsg.BuildRouter(modules.EdgeServiceModuleName, "", "", constants.ResponseOperation)
	beehiveContext.SendToGroup(modules.EdgeHubGroup, *responseMsg)
}

func buildErrorResponse(parentID string, content string, statusCode int) (model.Message, error) {
	responseMsg := model.NewMessage(parentID)
	c := httpservice.HTTPResponse{Header: nil, StatusCode: statusCode, Body: []byte(content)}
	responseMsg.Content = c
	responseMsg.BuildRouter(modules.EdgeServiceModuleName, "", "", constants.ResponseOperation)
	return *responseMsg, nil
}
