package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/namespace"

	"github.com/spotahome/kooper/log"
)

// Main runs the main application.
func Main() error {
	// Initialize logger.
	logger := &log.Std{}

	// Get k8s client.
	k8scfg, err := rest.InClusterConfig()
	if err != nil {
		// No in cluster? letr's try locally
		kubehome := filepath.Join(homedir.HomeDir(), ".kube", "config")
		k8scfg, err = clientcmd.BuildConfigFromFlags("", kubehome)
		if err != nil {
			return fmt.Errorf("error loading kubernetes configuration: %s", err)
		}
	}
	k8scli, err := kubernetes.NewForConfig(k8scfg)
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %s", err)
	}

	sc := NewServiceController(k8scli, namespace.Namespace(), logger)

	stopC := make(chan struct{})
	errC := make(chan error)
	go func() {
		errC <- sc.Run(stopC)
	}()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errC:
		if err != nil {
			logger.Infof("controller finished with error: %s", err)
			return err
		}
		logger.Infof("controller finished successfuly")
	case s := <-sigC:
		logger.Infof("signal %s received", s)
		close(stopC)
	}

	time.Sleep(5 * time.Second)

	return nil
}

func main() {
	if err := Main(); err != nil {
		fmt.Fprintf(os.Stderr, "error executing controller: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func KnetLabels() map[string]string {
	labels := make(map[string]string)
	labels["app"] = "knetblackmon"
	return labels
}
