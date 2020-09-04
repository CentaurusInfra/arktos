package main

import (
	"fmt"
	"os"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloudgateway/app"
)

func main() {
	command := app.NewCloudGatewayCommand()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
