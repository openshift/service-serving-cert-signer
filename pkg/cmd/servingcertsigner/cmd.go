package servingcertsigner

import (
	"github.com/spf13/cobra"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert/starter"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

const (
	componentName      = "openshift-service-serving-cert-signer-serving-ca"
	componentNamespace = "openshift-service-cert-signer"
)

func NewController() *cobra.Command {
	cmd := controllercmd.NewControllerCommandConfig(componentName, componentNamespace, version.Get(),
		starter.RunServingCert)
	cmd.Use = "serving-cert-signer"
	cmd.Short = "Start the Service Serving Cert Signer controller"
	return cmd
}
