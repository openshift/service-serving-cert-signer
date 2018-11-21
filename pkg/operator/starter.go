package operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsclient "github.com/openshift/client-go/servicecertsigner/clientset/versioned"
	scsinformers "github.com/openshift/client-go/servicecertsigner/informers/externalversions"
	"github.com/openshift/library-go/pkg/operator/status"
)

func RunOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	scsClient, err := scsclient.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	operatorInformers := scsinformers.NewSharedInformerFactory(scsClient, 10*time.Minute)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, 10*time.Minute, targetNamespaceName, nil)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		"openshift-service-cert-signer",
		"openshift-service-cert-signer",
		dynamicClient,
		&operatorStatusProvider{informers: operatorInformers},
	)

	operator := NewServiceCertSignerOperator(
		operatorInformers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs(),
		kubeInformersNamespaced,
		scsClient.ServicecertsignerV1alpha1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
		kubeClient.RbacV1(),
	)

	operatorInformers.Start(stopCh)
	kubeInformersNamespaced.Start(stopCh)

	go operator.Run(stopCh)
	go clusterOperatorStatus.Run(1, stopCh)

	<-stopCh
	return fmt.Errorf("stopped")
}

type operatorStatusProvider struct {
	informers scsinformers.SharedInformerFactory
}

func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
	return p.informers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs().Informer()
}

func (p *operatorStatusProvider) CurrentStatus() (operatorv1.OperatorStatus, error) {
	instance, err := p.informers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs().Lister().Get("instance")
	if err != nil {
		return operatorv1.OperatorStatus{}, err
	}
	// TODO need to move to operatorv1.OperatorStatus and drop this conversion
	in := instance.Status.OperatorStatus
	var conditions []operatorv1.OperatorCondition
	for _, condition := range in.Conditions {
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:               condition.Type,
			Status:             operatorv1.ConditionStatus(condition.Status),
			LastTransitionTime: condition.LastTransitionTime,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}
	var availability operatorv1alpha1.VersionAvailability
	if current := in.CurrentAvailability; current != nil {
		availability = *current
	}
	out := operatorv1.OperatorStatus{
		ObservedGeneration: in.ObservedGeneration,
		Conditions:         conditions,
		Version:            availability.Version,
		ReadyReplicas:      availability.ReadyReplicas,
		Generations:        availabilityToGenerations(in.CurrentAvailability),
	}
	return out, nil
}

func availabilityToGenerations(in *operatorv1alpha1.VersionAvailability) []operatorv1.GenerationStatus {
	var availability operatorv1alpha1.VersionAvailability
	if current := in; current != nil {
		availability = *current
	}
	var generations []operatorv1.GenerationStatus
	for _, generation := range availability.Generations {
		generations = append(generations, operatorv1.GenerationStatus{
			Group:          generation.Group,
			Resource:       generation.Resource,
			Namespace:      generation.Namespace,
			Name:           generation.Name,
			LastGeneration: generation.LastGeneration,
			Hash:           "",
		})
	}
	return generations
}
