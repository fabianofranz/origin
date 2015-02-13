package cmd

import (
	"fmt"
	"net/http"
	"os"
)

const (
	unauthorizedExitCode = 1

	unauthorizedErrorMessage = `Your login session has expired. Use the following command to log in again:

  openshift ex login [--username=<username>] [--password=<password>]
`
)

type statusHandlerClient struct {
	delegate *http.Client
}

func (client *statusHandlerClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := client.delegate.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Print(unauthorizedErrorMessage)
		os.Exit(unauthorizedExitCode)
	}
	return resp, err
}
