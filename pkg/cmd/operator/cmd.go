package operator

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

func NewOperator() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("openshift-service-cert-signer-operator", version.Get(), Run).
		NewCommand()
	cmd.Use = "operator"
	cmd.Short = "Start the Service Cert Signer Operator"

	return cmd
}

func Run(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	glog.Info("started operator")

	// TODO call out to do something useful

	<-stopCh
	return fmt.Errorf("stopped")
}
