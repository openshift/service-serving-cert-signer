package controllercmd

import (
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
)

func NewControllerCommandConfig(componentName, componentNamespace string, version version.Info, startFunc controllercmd.StartFunc) *cobra.Command {
	// TODO componentNamespace should be used
	cmd := controllercmd.NewControllerCommandConfig(componentName, version,
		withComponentName(startFunc, componentName),
	).NewCommand()
	// TODO this is wrong for the operator but correct for the controllers
	if err := cmd.MarkFlagRequired("config"); err != nil {
		panic(err) // this should never happen
	}
	return cmd
}

func withComponentName(startFunc controllercmd.StartFunc, componentName string) controllercmd.StartFunc {
	return func(unstructuredConfig *unstructured.Unstructured, kubeConfig *rest.Config, stopCh <-chan struct{}) error {
		newConfig := rest.CopyConfig(kubeConfig)
		newConfig.UserAgent = componentName + " " + rest.DefaultKubernetesUserAgent()
		return startFunc(unstructuredConfig, newConfig, stopCh)
	}
}
