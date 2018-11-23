package controllercmd

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func FromUnstructured(unstructuredConfig *unstructured.Unstructured, gv schema.GroupVersion, config runtime.Object) error {
	if unstructuredConfig == nil {
		return fmt.Errorf("config is required")
	}

	gvk := gv.WithKind(reflect.TypeOf(config).Elem().Name())
	if actualGVK := unstructuredConfig.GroupVersionKind(); actualGVK != gvk {
		return fmt.Errorf("invalid config type, expected %s, got %s", gvk, actualGVK)
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredConfig.Object, config)
}
