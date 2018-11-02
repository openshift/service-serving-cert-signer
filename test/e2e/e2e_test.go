package e2e

import (
	"crypto"
	crand "crypto/rand"
	"errors"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"

	"io/ioutil"

	cryptohelpers "github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert"
)

const (
	serviceCAOperatorNamespace   = "openshift-core-operators"
	serviceCAControllerNamespace = "openshift-service-cert-signer"
	serviceCAOperatorPodPrefix   = "openshift-service-cert-signer-operator"
	apiInjectorPodPrefix         = "apiservice-cabundle-injector"
	configMapInjectorPodPrefix   = "configmap-cabundle-injector"
	caControllerPodPrefix        = "service-serving-cert-signer"
)

func hasPodWithPrefixName(client *kubernetes.Clientset, name, namespace string) bool {
	if client == nil || len(name) == 0 || len(namespace) == 0 {
		return false
	}
	pods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.GetName(), name) {
			return true
		}
	}
	return false
}

func createTestNamespace(client *kubernetes.Clientset, namespaceName string) (*v1.Namespace, error) {
	ns, err := client.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	})
	return ns, err
}

// on success returns serviceName, secretName, nil
func createServingCertAnnotatedService(client *kubernetes.Clientset, secretName, serviceName, namespace string) error {
	_, err := client.CoreV1().Services(namespace).Create(&v1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Annotations: map[string]string{
				servingcert.ServingCertSecretAnnotation: secretName,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "tests",
					Port: 8443,
				},
			},
		},
	})
	return err
}

func getTLSCredsFromSecret(client *kubernetes.Clientset, secretName, namespace, certDataKey, keyDataKey string) ([]byte, []byte, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	certData, ok := secret.Data[certDataKey]
	if !ok {
		return nil, nil, fmt.Errorf("secret %s does not have data key %s", secret.Name, certDataKey)
	}
	if len(certData) == 0 {
		return nil, nil, fmt.Errorf("secret %s does not contain cert data", secret.Name)
	}
	keyData, ok := secret.Data[keyDataKey]
	if !ok {
		return nil, nil, fmt.Errorf("secret %s does not have data key %s", secret.Name, keyDataKey)
	}
	if len(keyData) == 0 {
		return nil, nil, fmt.Errorf("secret %s does not contain key data", secret.Name)
	}
	return certData, keyData, nil
}

func pollForServiceServingSecret(client *kubernetes.Clientset, secretName, namespace string) error {
	return wait.PollImmediate(time.Second, 10*time.Second, func() (bool, error) {
		_, err := client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil && kapierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
}

func cleanupServiceSignerTestObjects(client *kubernetes.Clientset, secretName, serviceName, namespace string) {
	client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	client.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{})
	client.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
}

func TestE2E(t *testing.T) {
	// use /tmp/admin.conf (placed by ci-operator) or KUBECONFIG env
	confPath := "/tmp/admin.conf"
	if conf := os.Getenv("KUBECONFIG"); conf != "" {
		confPath = conf
	}

	// load client
	client, err := clientcmd.LoadFromFile(confPath)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}
	adminConfig, err := clientcmd.NewDefaultClientConfig(*client, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		t.Fatalf("error loading admin config: %v", err)
	}
	adminClient, err := kubernetes.NewForConfig(adminConfig)
	if err != nil {
		t.Fatalf("error getting admin client: %v", err)
	}

	// the service-serving-cert-operator and controllers should be running as a stock OpenShift component. our first test is to
	// verify that all of the components are running.
	if !hasPodWithPrefixName(adminClient, serviceCAOperatorPodPrefix, serviceCAOperatorNamespace) {
		t.Fatalf("%s not running in %s namespace", serviceCAOperatorPodPrefix, serviceCAOperatorNamespace)
	}
	if !hasPodWithPrefixName(adminClient, apiInjectorPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", apiInjectorPodPrefix, serviceCAControllerNamespace)
	}
	if !hasPodWithPrefixName(adminClient, configMapInjectorPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", configMapInjectorPodPrefix, serviceCAControllerNamespace)
	}
	if !hasPodWithPrefixName(adminClient, caControllerPodPrefix, serviceCAControllerNamespace) {
		t.Fatalf("%s not running in %s namespace", caControllerPodPrefix, serviceCAControllerNamespace)
	}

	// test the main feature. annotate service -> created secret
	t.Run("serving-cert-annotation", func(t *testing.T) {
		ns, err := createTestNamespace(adminClient, "test-"+randSeq(5))
		if err != nil {
			t.Fatalf("could not create test namespace: %v", err)
		}
		testServiceName := "test-service-" + randSeq(5)
		testSecretName := "test-secret-" + randSeq(5)
		defer cleanupServiceSignerTestObjects(adminClient, testSecretName, testServiceName, ns.Name)

		err = createServingCertAnnotatedService(adminClient, testSecretName, testServiceName, ns.Name)
		if err != nil {
			t.Fatalf("error creating annotated service: %v", err)
		}

		err = pollForServiceServingSecret(adminClient, testSecretName, ns.Name)
		if err != nil {
			t.Fatalf("error fetching created serving cert secret: %v", err)
		}
	})

	t.Run("rotate CA", func(t *testing.T) {
		ns, err := createTestNamespace(adminClient, "test-"+randSeq(5))
		if err != nil {
			t.Fatalf("could not create test namespace: %v", err)
		}
		testServiceName := "test-service-" + randSeq(5)
		testSecretName := "test-secret-" + randSeq(5)
		defer cleanupServiceSignerTestObjects(adminClient, testSecretName, testServiceName, ns.Name)

		err = createServingCertAnnotatedService(adminClient, testSecretName, testServiceName, ns.Name)
		if err != nil {
			t.Fatalf("error creating annotated service: %v", err)
		}

		err = pollForServiceServingSecret(adminClient, testSecretName, ns.Name)
		if err != nil {
			t.Fatalf("error fetching created serving cert secret: %v", err)
		}

		//currentServingCert, currentServingKey, err := getTLSCredsFromSecret(adminClient, testSecretName, ns.Name, "tls.crt", "tls.key")
		//if err != nil {
		//	t.Fatalf("error fetching serving cert: %v", err)
		//}

		replacementSubject, err := getSigningSubject(adminClient)
		if err != nil {
			t.Fatalf("error fetching signing key subject: %v", err)
		}

		err = createServiceSignerReplacementCASecret(adminClient, replacementSubject, 100)
		if err != nil {
			t.Fatalf("error creating replacement ca: %v", err)
		}

		// make sure it's created
		err = pollForServiceServingSecret(adminClient, "next-service-signer", "openshift-service-cert-signer")
		if err != nil {
			t.Fatalf("error fetching replacement CA secret")
		}
		defer adminClient.CoreV1().Secrets("openshift-service-cert-signer").Delete("next-service-signer", nil)

		err = CreateCrossSignedInterimCAs(adminClient,
			"service-serving-cert-signer-signing-key",
			"openshift-service-cert-signer",
			"next-service-signer",
			"openshift-service-cert-signer",
		)
		if err != nil {
			t.Fatalf("error creating cross-signed certs: %v", err)
		}
	})
	// TODO: additional tests
	// - configmap CA bundle injection
	// - API service CA bundle injection
	// - regenerate serving cert
}

func getSigningSubject(client *kubernetes.Clientset) (pkix.Name, error) {
	currentCASecret, err := client.CoreV1().Secrets("openshift-service-cert-signer").Get("service-serving-cert-signer-signing-key", metav1.GetOptions{})
	if err != nil {
		return pkix.Name{}, err
	}
	caPem, ok := currentCASecret.Data["tls.crt"]
	if !ok {
		return pkix.Name{}, fmt.Errorf("no tls.crt data in service-serving-cert-signer-signing-key secret")
	}
	dataBlock, _ := pem.Decode(caPem)
	caCert, err := x509.ParseCertificate(dataBlock.Bytes)
	if err != nil {
		return pkix.Name{}, err
	}
	return caCert.Subject, nil
}

func CreateCrossSignedInterimCAs(client *kubernetes.Clientset, currentCASecretName, currentCASecretNamespace, newCASecretName, newCASecretNamespace string) error {
	curCACertDer, curCAKeyDer, err := getTLSCredsFromSecret(client, currentCASecretName, currentCASecretNamespace, "tls.crt", "tls.key")
	if err != nil {
		return err
	}
	newCACertDer, newCAKeyDer, err := getTLSCredsFromSecret(client, newCASecretName, newCASecretNamespace, "tls.crt", "tls.key")
	if err != nil {
		return err
	}

	curCABlock, _ := pem.Decode(curCACertDer)
	curCACert, err := x509.ParseCertificate(curCABlock.Bytes)
	if err != nil {
		return err
	}
	curCAKeyBlock, _ := pem.Decode(curCAKeyDer)
	curCAKey, err := x509.ParsePKCS1PrivateKey(curCAKeyBlock.Bytes)
	if err != nil {
		return err
	}
	newCABlock, _ := pem.Decode(newCACertDer)
	newCACert, err := x509.ParseCertificate(newCABlock.Bytes)
	if err != nil {
		return err
	}
	newCAKeyBlock, _ := pem.Decode(newCAKeyDer)
	newCAKey, err := x509.ParsePKCS1PrivateKey(newCAKeyBlock.Bytes)
	if err != nil {
		return err
	}

	// The first cross-signed intermediate has the current CA's public and private key and subject, signed by the new CA key
	// XXX change auth key ID to new CA auth key
	firstCrossSigned, err := x509.CreateCertificate(crand.Reader, curCACert, curCACert, curCACert.PublicKey, newCAKey)
	if err != nil {
		return err
	}

	firstCrossSignedCert, err := x509.ParseCertificates(firstCrossSigned)
	if err != nil {
		return err
	}
	if len(firstCrossSignedCert) != 1 {
		return fmt.Errorf("Expected one certificate")
	}

	firstCrossSignedCApem, err := encodeCertificates(firstCrossSignedCert...)
	if err != nil {
		return err
	}

	// XXX
	curCAPem, err := encodeCertificates(curCACert)
	if err != nil {
		return err
	}
	newCAPem, err := encodeCertificates(newCACert)
	if err != nil {
		return err
	}
	fmt.Printf("current CA PEM\n")
	fmt.Printf("%s\n", curCAPem)
	ioutil.WriteFile("/tmp/current.crt", curCAPem, 0644)
	fmt.Printf("new CA PEM\n")
	fmt.Printf("%s\n", newCAPem)
	ioutil.WriteFile("/tmp/new.crt", newCAPem, 0644)
	fmt.Printf("first cross signed PEM\n")
	fmt.Printf("%s\n", firstCrossSignedCApem)
	ioutil.WriteFile("/tmp/first.crt", firstCrossSignedCApem, 0644)

	// The second cross-signed intermediate has the new CA's public and private key and subject, signed by the old CA key
	secondCrossSigned, err := x509.CreateCertificate(crand.Reader, newCACert, newCACert, newCACert.PublicKey, curCAKey)
	if err != nil {
		return err
	}

	secondCrossSignedCert, err := x509.ParseCertificates(secondCrossSigned)
	if err != nil {
		return err
	}
	if len(secondCrossSignedCert) != 1 {
		return fmt.Errorf("Expected one certificate")
	}

	secondCrossSignedCApem, err := encodeCertificates(secondCrossSignedCert...)
	if err != nil {
		return err
	}

	// XXX
	fmt.Printf("second cross signed PEM\n")
	fmt.Printf("%s\n", secondCrossSignedCApem)
	ioutil.WriteFile("/tmp/second.crt", secondCrossSignedCApem, 0644)

	return nil
}

func createServiceSignerReplacementCASecret(adminClient *kubernetes.Clientset, caSubject pkix.Name, days int) error {
	// XXX set subjectKeyId
	replacementCATemplate := &x509.Certificate{
		Subject: caSubject,

		SignatureAlgorithm: x509.SHA256WithRSA,

		NotBefore:    time.Now().Add(-1 * time.Second),
		NotAfter:     time.Now().Add(time.Duration(days) * 24 * time.Hour),
		SerialNumber: big.NewInt(1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}

	replacementCAPublicKey, replacementCAPrivateKey, err := cryptohelpers.NewKeyPair()
	if err != nil {
		return err
	}

	replacementDer, err := x509.CreateCertificate(crand.Reader, replacementCATemplate, replacementCATemplate, replacementCAPublicKey, replacementCAPrivateKey)
	if err != nil {
		return err
	}

	replacementCert, err := x509.ParseCertificates(replacementDer)
	if err != nil {
		return err
	}
	if len(replacementCert) != 1 {
		return fmt.Errorf("Expected one certificate")
	}

	caPem, err := encodeCertificates(replacementCert...)
	if err != nil {
		return err
	}

	caKey, err := encodeKey(replacementCAPrivateKey)
	if err != nil {
		return err
	}

	_, err = adminClient.CoreV1().Secrets("openshift-service-cert-signer").Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "next-service-signer",
		},
		Data: map[string][]byte{
			"tls.crt": caPem,
			"tls.key": caKey,
		},
		Type: "kubernetes.io/tls",
	})
	return err
}

func encodeCertificates(certs ...*x509.Certificate) ([]byte, error) {
	b := bytes.Buffer{}
	for _, cert := range certs {
		if err := pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
			return []byte{}, err
		}
	}
	return b.Bytes(), nil
}

func encodeKey(key crypto.PrivateKey) ([]byte, error) {
	b := bytes.Buffer{}
	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return []byte{}, err
		}
		if err := pem.Encode(&b, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
			return b.Bytes(), err
		}
	case *rsa.PrivateKey:
		if err := pem.Encode(&b, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
			return []byte{}, err
		}
	default:
		return []byte{}, errors.New("Unrecognized key type")

	}
	return b.Bytes(), nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

var characters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// used for random suffix
func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}
