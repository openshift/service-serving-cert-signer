package apiservicecabundle

import (
	"github.com/spf13/cobra"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/apiservicecabundle/starter"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-apiservice-injector"
	componentNamespace = "openshift-service-cert-signer"
)

func NewController() *cobra.Command {
	cmd := controllercmd.NewControllerCommandConfig(componentName, componentNamespace, version.Get(),
		starter.RunAPIServiceCABundleInjector)
	cmd.Use = "apiservice-cabundle-injector"
	cmd.Short = "Start the APIService CA Bundle Injection controller"
	return cmd
}
