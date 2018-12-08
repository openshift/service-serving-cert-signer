package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"

	"bytes"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

// saTokenCABundleInjectionController is responsible for injecting a CA bundle into service account token secrets.
type saTokenCABundleInjectionController struct {
	secretsClient kcoreclient.SecretsGetter
	secretsLister listers.SecretLister

	ca []byte
}

func secretIsSAToken(obj v1.Object) bool {
	return obj.(*corev1.Secret).Type == corev1.SecretTypeServiceAccountToken
}

func NewSATokenCABundleInjectionController(secrets informers.SecretInformer, secretsClient kcoreclient.SecretsGetter, ca string) controller.Runner {
	tc := &saTokenCABundleInjectionController{
		secretsClient: secretsClient,
		secretsLister: secrets.Lister(),
		ca:            []byte(ca),
	}

	return controller.New("SATokenCABundleInjectionController", tc,
		controller.WithInformer(secrets, controller.FilterFuncs{
			AddFunc: secretIsSAToken,
			UpdateFunc: func(old, cur v1.Object) bool {
				return secretIsSAToken(cur)
			},
		}),
	)
}

func (tc *saTokenCABundleInjectionController) Key(namespace, name string) (v1.Object, error) {
	return tc.secretsLister.Secrets(namespace).Get(name)
}

func (tc *saTokenCABundleInjectionController) Sync(obj v1.Object) error {
	secret := obj.(*corev1.Secret)

	// skip updating when the CA bundle is already there and the same
	if data, ok := secret.Data[api.InjectionDataKey]; ok && bytes.Equal(data, tc.ca) {
		return nil
	}

	// make a copy to avoid mutating cache state
	secretCopy := secret.DeepCopy()

	if secretCopy.Data == nil {
		secretCopy.Data = map[string][]byte{}
	}
	secretCopy.Data[api.InjectionDataKey] = tc.ca

	_, err := tc.secretsClient.Secrets(secretCopy.Namespace).Update(secretCopy)
	return err
}
