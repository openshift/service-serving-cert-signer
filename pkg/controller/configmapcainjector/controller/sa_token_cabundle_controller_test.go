package controller

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/kubernetes/fake"
	listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

func TestSyncSATokenCABundle(t *testing.T) {
	tests := []struct {
		name            string
		startingSecrets []runtime.Object
		namespace       string
		secretName      string
		caBundle        string
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:       "missing",
			namespace:  "foo",
			secretName: "foo",
			caBundle:   "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
		{
			name: "requested and empty",
			startingSecrets: []runtime.Object{
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "foo",
					},
					Data: map[string][]byte{},
					Type: v1.SecretTypeServiceAccountToken,
				},
			},
			namespace:  "foo",
			secretName: "foo",
			caBundle:   "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "secrets") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*v1.Secret)
				if expected := "content"; string(actual.Data[api.InjectionDataKey]) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
		{
			name: "requested and different",
			startingSecrets: []runtime.Object{
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "foo",
					},
					Data: map[string][]byte{
						api.InjectionDataKey: []byte("foo"),
					},
					Type: v1.SecretTypeServiceAccountToken,
				},
			},
			namespace:  "foo",
			secretName: "foo",
			caBundle:   "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("update", "secrets") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[0].(clienttesting.UpdateAction).GetObject().(*v1.Secret)
				if expected := "content"; string(actual.Data[api.InjectionDataKey]) != expected {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
		},
		{
			name: "requested and same",
			startingSecrets: []runtime.Object{
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "foo",
					},
					Data: map[string][]byte{
						api.InjectionDataKey: []byte("content"),
					},
					Type: v1.SecretTypeServiceAccountToken,
				},
			},
			namespace:  "foo",
			secretName: "foo",
			caBundle:   "content",
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Fatal(spew.Sdump(actions))
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tc.startingSecrets...)
			index := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, secret := range tc.startingSecrets {
				index.Add(secret)
			}
			c := &saTokenCABundleInjectionController{
				secretsLister: listers.NewSecretLister(index),
				secretsClient: fakeClient.CoreV1(),
				ca:            []byte(tc.caBundle),
			}

			obj, err := c.Key(tc.namespace, tc.secretName)
			if err == nil {
				if err := c.Sync(obj); err != nil {
					t.Fatal(err)
				}
			}

			tc.validateActions(t, fakeClient.Actions())
		})
	}
}
