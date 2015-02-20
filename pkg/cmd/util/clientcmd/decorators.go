package clientcmd

import (
	"fmt"
	"strings"
)

func DecorateErrors(err error) error {
	err = DecorateNoServerFoundError(err)
	err = DecorateServerCertificateIssues(err)
	return err
}

func DecorateNoServerFoundError(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "no server found for") { // TODO there must be a better way to detect this  (probably use custom type for error)
			// TODO parameterize parent cmd name
			return fmt.Errorf(`OpenShift is not configured. You need to run the login command in order to create a default config for your server and credentials:

  osc login [--username=<username>] [--password<password>] [--server=<server>]

You can also run this command again providing the path to a config file directly, either through the --config flag of the OPENSHIFTCONFIG environment variable.
`)
		}
	}
	return err
}

// TODO yikes refactor (probably use custom type for error)
func DecorateServerCertificateIssues(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "certificate signed by unknown authority") {
			return fmt.Errorf("The server uses a certificate signed by an unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections.")
		}
	}
	return err
}
