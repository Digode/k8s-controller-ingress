package controller

import (
	"context"
	"fmt"
	"log"
	"strings"

	"ingress-controller/internal/configs"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeobj "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type DeployWatcher struct {
	clientset *kubernetes.Clientset
	queue     workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
}

var appConfig = configs.Get()

func NewDeployWatcher() *DeployWatcher {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error to configure client: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error to create client: %v", err)
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtimeobj.Object, error) {
				return clientset.AppsV1().Deployments("").List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.AppsV1().Deployments("").Watch(context.Background(), options)
			},
		},
		&appsv1.Deployment{},
		0,
		cache.Indexers{},
	)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return &DeployWatcher{
		clientset: clientset,
		queue:     queue,
		informer:  informer,
	}
}

func (c *DeployWatcher) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Printf("Creating deployment: %v", obj.(*appsv1.Deployment).Name)
			createObjects(obj, c.clientset)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			log.Printf("Updating deployment from: %v, to %v", oldObj.(*appsv1.Deployment).Name, newObj.(*appsv1.Deployment).Name)
			updateDeploy(oldObj, newObj, c.clientset)
		},
		DeleteFunc: func(obj interface{}) {
			log.Printf("Deleting deployment: %v", obj.(*appsv1.Deployment).Name)
			deleteObjects(obj, c.clientset)
		},
	})

	go c.informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		log.Fatal("Fail to cache sync")
	}

	<-stopCh
}

func createObjects(obj interface{}, clientset *kubernetes.Clientset) {
	deployment := obj.(*appsv1.Deployment)
	annotations := deployment.GetAnnotations()
	labels := deployment.GetLabels()
	annotationValue, ok := annotations[appConfig.Annotation.Key]
	labelValue, okLabel := labels[appConfig.LabelPath]
	path := "/"

	if okLabel {
		path = labelValue
	}

	if ok {
		apigtws := strings.Split(annotationValue, ",")
		for _, apigtw := range apigtws {

			if validateValueOfAnnotation(apigtw, deployment) && validateLabelSubdomain(deployment) {
				service := createService(deployment, clientset)
				createIngress(deployment, service, apigtw, clientset, path)
			} else {
				log.Printf("The annotation value not is valid to create objects %s/%s", deployment.Namespace, deployment.Name)
			}
		}
	}
}

func updateDeploy(oldObj interface{}, newObj interface{}, clientset *kubernetes.Clientset) {
	oldDeployment := oldObj.(*appsv1.Deployment)
	oldAnnotations := oldDeployment.GetAnnotations()
	oldLabels := oldDeployment.GetLabels()
	oldApigtw, oldApigtwOK := oldAnnotations[appConfig.Annotation.Key]
	oldLabel, oldLabelOK := oldLabels[appConfig.LabelSubdomain]

	newDeployment := newObj.(*appsv1.Deployment)
	newAnnotations := newDeployment.GetAnnotations()
	newLabels := newDeployment.GetLabels()
	newApigtw, newApigtwOK := newAnnotations[appConfig.Annotation.Key]
	newLabel, newLabelOK := newLabels[appConfig.LabelSubdomain]

	if (!oldApigtwOK && newApigtwOK && oldLabelOK && newLabelOK) && validateValueOfAnnotation(newApigtw, newDeployment) && validateLabelSubdomain(newDeployment) {
		createObjects(newObj, clientset)
	} else if (oldApigtwOK && newApigtwOK && oldLabelOK && newLabelOK) && (oldApigtw != newApigtw || oldLabel != newLabel) && validateValueOfAnnotation(newApigtw, newDeployment) && validateLabelSubdomain(newDeployment) {
		deleteObjects(oldObj, clientset)
		createObjects(newObj, clientset)
	} else if oldApigtwOK && !newApigtwOK {
		deleteObjects(oldObj, clientset)
	} else {
		log.Printf("Deployment %s/%s not is valid for the rule update, old: %v, new: %v", oldDeployment.Namespace, oldDeployment.Name, oldApigtwOK, newApigtwOK)
	}
}

func deleteObjects(obj interface{}, clientset *kubernetes.Clientset) {
	deployment := obj.(*appsv1.Deployment)
	annotations := deployment.GetAnnotations()
	apigtw, ok := annotations[appConfig.Annotation.Key]

	if ok {
		serviceName := deployment.Name + "-svc"

		err := clientset.CoreV1().Services(deployment.Namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})

		if err != nil {
			log.Printf("Error deleting Service %s: %v", serviceName, err)
		} else {
			log.Printf("Service %s deleted successfully", serviceName)
		}

		dnsZones := strings.Split(apigtw, ",")
		for _, dnsZone := range dnsZones {
			ingressName := getIngressName(deployment, dnsZone)
			err = clientset.NetworkingV1().Ingresses(deployment.Namespace).Delete(context.Background(), ingressName, metav1.DeleteOptions{})

			if err != nil {
				log.Printf("Error deleting Ingress %s: %v", ingressName, err)
			} else {
				log.Printf("Ingress %s deleted successfully", ingressName)
			}
		}
	}
}

func createService(deployment *appsv1.Deployment, clientset *kubernetes.Clientset) *corev1.Service {
	serviceName := deployment.Name + "-svc"
	var service *corev1.Service
	var err error

	service, err = clientset.CoreV1().Services(deployment.Namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Printf("Error getting service %s: %v", serviceName, err)
			return service
		}
		service = nil
	}

	if service != nil {
		log.Printf("Service already created: %v", serviceName)
		return service
	}

	port := deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: deployment.Namespace,
			Labels:    deployment.Labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: deployment.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(int(port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	service, err = clientset.CoreV1().Services(deployment.Namespace).Create(context.Background(), service, metav1.CreateOptions{})

	if err != nil {
		log.Printf("Error creating service: %v", err)
		return nil
	}
	log.Printf("Service created: %v", service.Name)

	return service
}

func getIngressName(deployment *appsv1.Deployment, dnsZone string) string {
	return deployment.Name + "-" + dnsZone[0:3] + "-ing"
}

func createIngress(deployment *appsv1.Deployment, service *corev1.Service, dnsZone string, clientset *kubernetes.Clientset, path string) *networkingv1.Ingress {
	if service == nil {
		log.Printf("Service not exists to create Ingress: %v", service.Name)
		return nil
	}

	ingressName := getIngressName(deployment, dnsZone)
	var ingress *networkingv1.Ingress
	var err error

	ingress, err = clientset.NetworkingV1().Ingresses(deployment.Namespace).Get(context.Background(), ingressName, metav1.GetOptions{})

	if err != nil {
		if !errors.IsNotFound(err) {
			log.Printf("Error getting ingress: %v", ingressName)
			return nil
		}
		ingress = nil
	}

	if ingress != nil {
		log.Printf("Ingress already created: %v", ingressName)
		return ingress
	}

	className := dnsZone + appConfig.IngressClassNameSuffix
	ingress = &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: service.Namespace,
			Labels:    service.Spec.Selector,
			Annotations: map[string]string{
				"dns_zone": dnsZone,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules:            getIngressRules(service, dnsZone, path),
			TLS:              getTls(service, dnsZone),
		},
	}

	ingress, err = clientset.NetworkingV1().Ingresses(deployment.Namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error creating ingress: %v", err)
		return nil
	}
	log.Printf("Ingress created: %v", ingress.Name)

	return ingress
}

func getIngressRules(service *corev1.Service, dnsZone string, path string) []networkingv1.IngressRule {
	var ingressRules = []networkingv1.IngressRule{}
	pathType := networkingv1.PathTypePrefix

	for _, domain := range getHosts(dnsZone) {
		ingressRules = append(ingressRules, networkingv1.IngressRule{
			Host: getIngressHost(service, domain),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: service.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: service.Spec.Ports[0].Port,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	return ingressRules
}

func getTls(service *corev1.Service, dnsZone string) []networkingv1.IngressTLS {
	if !appConfig.Tls {
		return nil
	}

	hosts := []string{}
	for _, domain := range getHosts(dnsZone) {
		hosts = append(hosts, getIngressHost(service, domain))
	}
	return []networkingv1.IngressTLS{{Hosts: hosts}}
}

func getHosts(annotation string) []string {
	if exists(annotation, appConfig.Annotation.Privates) {
		return appConfig.Domain.Privates
	} else {
		return appConfig.Domain.Publics
	}
}

func getIngressHost(service *corev1.Service, domain string) string {
	return service.Labels[appConfig.LabelSubdomain] + domain
}

func validateValueOfAnnotation(annotation string, deploy *appsv1.Deployment) bool {
	annSpt := strings.Split(annotation, ",")
	for _, s := range annSpt {
		if exists(s, appConfig.Annotation.Privates) {
			return true
		}
		if exists(s, appConfig.Annotation.Publics) {
			return true
		}
	}

	log.Printf("Rule: Value \"%s\" of annotation \"%s\" not is valid on Deployment %s/%s", annotation, appConfig.Annotation.Key, deploy.Namespace, deploy.Name)
	return false
}

func validateLabelSubdomain(deploy *appsv1.Deployment) bool {
	_, ok := deploy.Labels[appConfig.LabelSubdomain]

	if !ok {
		log.Printf(fmt.Sprintf("Rule: Labels not contains \"%s\" on Deployment %s/%s => Labels: %+v", appConfig.LabelSubdomain, deploy.Namespace, deploy.Name, deploy.Labels))
	}
	return ok
}

func exists(val string, list []string) bool {
	for _, item := range list {
		if val == item {
			return true
		}
	}
	return false
}
