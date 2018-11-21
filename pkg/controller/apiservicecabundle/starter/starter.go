package starter

import (
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/apiservicecabundle/controller"
)

func RunAPIServiceCABundleInjector(unstructuredConfig *unstructured.Unstructured, kubeConfig *rest.Config, stopCh <-chan struct{}) error {
	config := &servicecertsignerv1alpha1.APIServiceCABundleInjectorConfig{}
	if err := controllercmd.FromUnstructured(unstructuredConfig, servicecertsignerv1alpha1.GroupVersion, config); err != nil {
		return err
	}

	if len(config.CABundleFile) == 0 {
		return fmt.Errorf("no signing cert/key pair provided")
	}

	caBundleContent, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return err
	}

	apiServiceClient, err := apiserviceclient.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	apiServiceInformers := apiserviceinformer.NewSharedInformerFactory(apiServiceClient, 2*time.Minute)

	servingCertUpdateController := controller.NewAPIServiceCABundleInjector(
		apiServiceInformers.Apiregistration().V1().APIServices(),
		apiServiceClient.ApiregistrationV1(),
		caBundleContent,
	)

	apiServiceInformers.Start(stopCh)

	go servingCertUpdateController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
