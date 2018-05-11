package main

import (
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	namespace string
}

func NewServiceKiller(kubeCli kubernetes.Interface, namespace string, logger log.Logger) *ServiceKiller {
	return &ServiceKiller{k8sCli: kubeCli, namespace: namespace, logger: logger}
}

// Start will run the service killer at regular intervals.
func (p *ServiceKiller) Start() error {
	lesvc, err := leaderelection.NewDefault(leaderElectionKey, p.namespace, p.k8sCli, p.logger)
	if err != nil {
		return err
	}

	return lesvc.Run(p.run)
}

func (p *ServiceKiller) run() error {
	svc, err := p.generateService()
	if err != nil {
		return err
	}
	for {
		<-time.After(time.Duration(periodSeconds) * time.Second)
		if err := p.k8sCli.CoreV1().Services(p.namespace).Delete(svc.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
		svc, err = p.generateService()
		if err != nil {
			return err
		}

	}
}

func (p *ServiceKiller) generateService() (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprint("knetblackmon-", rand.Intn(99)),
			Labels: KnetLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   "TCP",
				},
			},
			Selector: KnetLabels(),
		},
	}

	p.k8sCli.CoreV1().Services(p.namespace).Delete(svc.Name, &metav1.DeleteOptions{})
	return p.k8sCli.CoreV1().Services(p.namespace).Create(svc)
}
