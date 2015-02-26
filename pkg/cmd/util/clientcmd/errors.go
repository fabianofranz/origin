package clientcmd

import "strings"

type ClientError struct {
	reason int
	msg    string
}

func (e ClientError) Error() string {
	return e.msg
}

const (
	notConfiguredReason               = 1
	certificateAuthorityUnknownReason = 2

	certificateAuthorityUnknownMsg = "The server uses a certificate signed by unknown authority. You may need to use the --certificate-authority flag to provide the path to a certificate file for the certificate authority, or --insecure-skip-tls-verify to bypass the certificate check and use insecure connections."
	notConfiguredMsg               = `OpenShift is not configured. You need to run the login command in order to create a default config for your server and credentials:
  osc login
You can also run this command again providing the path to a config file directly, either through the --config flag of the OPENSHIFTCONFIG environment variable.
`
)

func WrapClientErrors(err error) error {
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "certificate signed by unknown authority"):
			return ClientError{certificateAuthorityUnknownReason, certificateAuthorityUnknownMsg}

		case strings.Contains(err.Error(), "no server found for"):
			return ClientError{notConfiguredReason, notConfiguredMsg}
		}
	}
	return err
}

func (e *ClientError) IsNotConfigured() bool {
	return e.reason == notConfiguredReason
}

func (e *ClientError) IsCertificateAuthorityUnknown() bool {
	return e.reason == certificateAuthorityUnknownReason
}
