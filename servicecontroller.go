package main

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/cache"

	"github.com/spotahome/kooper/log"
	"github.com/spotahome/kooper/operator/controller"
	"github.com/spotahome/kooper/operator/handler"
	"github.com/spotahome/kooper/operator/retrieve"
)

type ServiceController struct {
	kubeCli   kubernetes.Interface
	logger    log.Logger
	namespace string

	endpointScraper *EndpointScraper
	stopC           chan struct{}
}

func NewServiceController(kubeCli kubernetes.Interface, namespace string, logger log.Logger) *ServiceController {
	stopC := make(chan struct{})
	return &ServiceController{kubeCli: kubeCli, namespace: namespace, logger: logger, stopC: stopC}
}

func (sc *ServiceController) Run(stopC chan struct{}) error {
	// Create our retriever so the controller knows how to get/listen for services events.
	servicesRetr := &retrieve.Resource{
		Object: &corev1.Service{},
		ListerWatcher: &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				set := labels.Set(KnetLabels())
				options.LabelSelector = set.String()
				return sc.kubeCli.CoreV1().Services("").List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				set := labels.Set(KnetLabels())
				options.LabelSelector = set.String()
				return sc.kubeCli.CoreV1().Services("").Watch(options)
			},
		},
	}

	// Our domain logic that will print every add/sync/update and delete event we .
	servicesHand := &handler.HandlerFunc{
		AddFunc: func(obj runtime.Object) error {
			srv := obj.(*corev1.Service)
			if sc.endpointScraper == nil || sc.endpointScraper.Service != srv {
				close(sc.stopC)
				sc.stopC = make(chan struct{})
				sc.endpointScraper = NewEndpointScraper(sc.kubeCli, sc.namespace, srv, sc.logger)
				go sc.endpointScraper.Run(sc.stopC)
				go func() {
					select {
					default:
						NewServiceKiller(sc.kubeCli, sc.namespace, srv, sc.logger).Start()
					case <-stopC:
						return
					}
				}()
			}

			sc.logger.Infof("Srv added: %s/%s", srv.Namespace, srv.Name)
			return nil
		},
		DeleteFunc: func(s string) error {
			close(sc.stopC)
			sc.logger.Infof("Service deleted: %s", s)
			return nil
		},
	}

	// // Leader election service.
	// lesvc, err := leaderelection.NewDefault(leaderElectionKey, sc.namespace, sc.kubeCli, sc.logger)
	// if err != nil {
	// 	return err
	// }

	// Create the controller and run.
	// cfg := &controller.Config{
	// 	ProcessingJobRetries: 5,
	// 	ResyncInterval:       time.Duration(30) * time.Second,
	// 	ConcurrentWorkers:    1,
	// }

	// Create metrics endpoint
	metricsRecorder := createPrometheusRecorder(sc.logger, sc.namespace)

	controller.NewSequential(30*time.Second, servicesHand, servicesRetr, metricsRecorder, sc.logger).Run(stopC)

	return nil
}
