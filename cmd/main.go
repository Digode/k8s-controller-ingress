package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"ingress-controller/internal/controller"
)

func main() {
	// Set up signal handling to gracefully shut down the controller
	stopCh := make(chan struct{})
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalCh
		close(stopCh)
	}()

	// Initialize the DeployWatcher
	deployWatcher := controller.NewDeployWatcher()

	// Start the DeployWatcher
	go deployWatcher.Run(stopCh)

	// Wait for shutdown signal
	<-stopCh
	log.Println("Shutting down the controller...")
}
