package main

import (
	"strings"
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

	endpointsScrapers map[string]*EndpointScraper
	stopChannels      map[string]chan struct{}
}

func NewServiceController(kubeCli kubernetes.Interface, namespace string, logger log.Logger) *ServiceController {
	stopC := make(map[string]chan struct{})
	es := make(map[string]*EndpointScraper)
	return &ServiceController{kubeCli: kubeCli, namespace: namespace, logger: logger, stopChannels: stopC, endpointsScrapers: es}
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
			if val, ok := sc.endpointsScrapers[srv.Name]; ok {
				if val.Service != srv {
					close(sc.stopChannels[srv.Name])
				} else {
					return nil
				}
			}
			sc.stopChannels[srv.Name] = make(chan struct{})
			sc.endpointsScrapers[srv.Name] = NewEndpointScraper(sc.kubeCli, sc.namespace, srv, sc.logger)
			go sc.endpointsScrapers[srv.Name].Run(sc.stopChannels[srv.Name])

			sc.logger.Infof("Srv added: %s/%s", srv.Namespace, srv.Name)
			return nil
		},
		DeleteFunc: func(s string) error {
			svc := strings.Split(s, "/")
			namespace := svc[0]
			name := svc[1]
			if c, ok := sc.stopChannels[name]; ok && sc.endpointsScrapers[name].Service.Namespace == namespace {
				close(c)
				sc.endpointsScrapers[s] = nil
			}
			sc.logger.Infof("Service deleted: %s", s)
			return nil
		},
	}

	// Create metrics endpoint
	metricsRecorder := createPrometheusRecorder(sc.logger, sc.namespace)

	controller.NewSequential(30*time.Second, servicesHand, servicesRetr, metricsRecorder, sc.logger).Run(stopC)

	return nil
}
