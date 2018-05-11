package main

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/spotahome/kooper/log"
	"github.com/spotahome/kooper/operator/controller/leaderelection"
)

const (
	leaderElectionKey        = "knetblackmon-servicekiller-leader"
	resyncIntervalSecondsDef = 30
	periodSeconds            = 60
)

// PodKiller will kill pods at regular intervals.
type ServiceKiller struct {
	k8sCli    kubernetes.Interface
	logger    log.Logger
	Service   *corev1.Service
	Namespace string
}

func NewServiceKiller(kubeCli kubernetes.Interface, namespace string, srv *corev1.Service, logger log.Logger) *ServiceKiller {
	return &ServiceKiller{k8sCli: kubeCli, Namespace: namespace, logger: logger, Service: srv}
}

// Start will run the service killer at regular intervals.
func (p *ServiceKiller) Start() error {
	lesvc, err := leaderelection.NewDefault(leaderElectionKey, p.Namespace, p.k8sCli, p.logger)
	if err != nil {
		return err
	}

	return lesvc.Run(p.run)
}

func (p *ServiceKiller) run() error {
	for {
		<-time.After(time.Duration(periodSeconds) * time.Second)
		p.logger.Infof("tick")
	}
}
