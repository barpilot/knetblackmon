package main

import (
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/spotahome/kooper/log"
	"github.com/spotahome/kooper/operator/controller"
	"github.com/spotahome/kooper/operator/handler"
	"github.com/spotahome/kooper/operator/retrieve"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type EndpointScraper struct {
	kubeCli   kubernetes.Interface
	logger    log.Logger
	namespace string
	Service   *corev1.Service
	endpoints corev1.Endpoints
	urls      []string
	stopC     chan struct{}
}

func NewEndpointScraper(kubeCli kubernetes.Interface, namespace string, srv *corev1.Service, logger log.Logger) *EndpointScraper {
	stopC := make(chan struct{})
	return &EndpointScraper{kubeCli: kubeCli, namespace: namespace, Service: srv, logger: logger, stopC: stopC}
}

func (eps *EndpointScraper) Run() {
	//eps.kubeCli.CoreV1().Endpoints(eps.namespace).Get(eps.service.Name, metav1.GetOptions{})

	retr := &retrieve.Resource{
		Object: &corev1.Endpoints{},
		ListerWatcher: &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return eps.kubeCli.CoreV1().Endpoints(eps.namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return eps.kubeCli.CoreV1().Endpoints(eps.namespace).Watch(options)
			},
		},
	}

	// Our domain logic that will print every add/sync/update and delete event we .
	hand := &handler.HandlerFunc{
		AddFunc: func(obj runtime.Object) error {
			ep := obj.(*corev1.Endpoints)
			if ep.Name != eps.Service.Name {
				return nil
			}
			eps.refreshEndpoints(ep)
			//log.Infof("ep added: %s/%s", ep.Namespace, ep.Name)
			return nil
		},
		DeleteFunc: func(s string) error {
			//log.Infof("ep deleted: %s", s)
			return nil
		},
	}

	// Create the controller that will refresh every 30 seconds.
	ctrl := controller.NewSequential(30*time.Second, hand, retr, nil, eps.logger)

	go ctrl.Run(eps.stopC)

	go eps.RunScrapper(eps.stopC)

	//eps.refreshEndpoints()
	//return eps
}

func (eps *EndpointScraper) Stop() {
	close(eps.stopC)
}

//&Endpoints{Subsets:[{[{172.17.0.4 } {172.17.0.5 }] [] [{4567 4567 TCP}]}],}

func (eps *EndpointScraper) refreshEndpoints(ep *corev1.Endpoints) {

	var urls []string
	for _, subset := range ep.Subsets {
		for _, address := range subset.Addresses {
			for _, subset := range ep.Subsets {
				for _, port := range subset.Ports {
					urls = append(urls, fmt.Sprintf("http://%s:%d", address.IP, port.Port))
				}
			}
		}
	}
	eps.endpoints = *ep
	eps.urls = urls
	return
}

func (eps *EndpointScraper) RunScrapper(stopC chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		<-stopC
		ticker.Stop()
	}()

	for range ticker.C {
		for _, url := range eps.urls {
			res, err := http.Head(url)
			if err != nil || res.StatusCode != http.StatusOK {
				eps.logger.Infof("error with url %s", url)
			}
		}
	}
}
