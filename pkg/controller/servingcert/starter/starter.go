package starter

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert/controller"
)

func RunServingCert(unstructuredConfig *unstructured.Unstructured, kubeConfig *rest.Config, stopCh <-chan struct{}) error {
	config := &servicecertsignerv1alpha1.ServiceServingCertSignerConfig{}
	if err := controllercmd.FromUnstructured(unstructuredConfig, servicecertsignerv1alpha1.GroupVersion, config); err != nil {
		return err
	}

	ca, err := crypto.GetCA(config.Signer.CertFile, config.Signer.KeyFile, "")
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 20*time.Minute)

	servingCertController := controller.NewServiceServingCertController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
	)
	servingCertUpdateController := controller.NewServiceServingCertUpdateController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
	)

	kubeInformers.Start(stopCh)

	go servingCertController.Run(5, stopCh)
	go servingCertUpdateController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
