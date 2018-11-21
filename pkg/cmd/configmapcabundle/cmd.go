package configmapcabundle

import (
	"github.com/spf13/cobra"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/configmapcainjector/starter"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-cabundle-injector"
	componentNamespace = "openshift-service-cert-signer"
)

func NewController() *cobra.Command {
	cmd := controllercmd.NewControllerCommandConfig(componentName, componentNamespace, version.Get(),
		starter.RunConfigMapCABundleInjector)
	cmd.Use = "configmap-cabundle-injector"
	cmd.Short = "Start the ConfigMap CA Bundle Injection controller"
	return cmd
}
