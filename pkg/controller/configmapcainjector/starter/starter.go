package starter

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/configmapcainjector/controller"
)

func RunConfigMapCABundleInjector(unstructuredConfig *unstructured.Unstructured, kubeConfig *rest.Config, stopCh <-chan struct{}) error {
	config := &servicecertsignerv1alpha1.ConfigMapCABundleInjectorConfig{}
	if err := controllercmd.FromUnstructured(unstructuredConfig, servicecertsignerv1alpha1.GroupVersion, config); err != nil {
		return err
	}

	if len(config.CABundleFile) == 0 {
		return fmt.Errorf("no ca bundle provided")
	}

	ca, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return err
	}

	// Verify that there is at least one cert in the bundle file
	block, _ := pem.Decode(ca)
	if block == nil {
		return fmt.Errorf("failed to parse CA bundle file as pem")
	}
	if _, err = x509.ParseCertificate(block.Bytes); err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 20*time.Minute)

	configMapInjectorController := controller.NewConfigMapCABundleInjectionController(
		kubeInformers.Core().V1().ConfigMaps(),
		kubeClient.CoreV1(),
		string(ca),
	)

	kubeInformers.Start(stopCh)

	go configMapInjectorController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
