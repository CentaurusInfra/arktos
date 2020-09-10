package main

import (
	"fmt"
	"os"

	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/edgegateway/app"
)

func main() {
	command := app.NewEdgeGatewayCommand()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
