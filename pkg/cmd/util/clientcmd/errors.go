package clientcmd

import (
	"fmt"
	"strings"
)

const (
	certificateSignedByUnknownAuthorityMsg = "The server uses a certificate signed by unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections."
	noServerFoundMsg                       = `OpenShift is not configured. You need to run the login command in order to create a default config for your server and credentials:
  osc login
You can also run this command again providing the path to a config file directly, either through the --config flag of the OPENSHIFTCONFIG environment variable.
`
)

func WrapClientErrors(err error) error {
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "certificate signed by unknown authority"):
			return fmt.Errorf(certificateSignedByUnknownAuthorityMsg)
		case strings.Contains(err.Error(), "no server found for"):
			return fmt.Errorf(noServerFoundMsg)
		}
	}
	return err
}
