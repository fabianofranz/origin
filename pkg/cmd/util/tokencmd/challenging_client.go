package tokencmd

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// challengingClient conforms the kclient.HTTPClient interface.  It introspects responses for auth challenges and
// tries to response to those challenges in order to get a token back.
type challengingClient struct {
	delegate            *http.Client
	reader              io.Reader
	defaultUsername     string
	defaultPassword     string
	defaultRoundTripper http.RoundTripper
}

const basicAuthPattern = `[\s]*Basic[\s]*realm="([\w]+)"`

var basicAuthRegex = regexp.MustCompile(basicAuthPattern)

// Do watches for unauthorized challenges.  If we know to respond, we respond to the challenge
func (client *challengingClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := client.delegate.Do(req)
	if err != nil {
		err = clientcmd.WrapClientErrors(err)
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		if wantsBasicAuth, realm := isBasicAuthChallenge(resp); wantsBasicAuth {
			username := client.defaultUsername
			password := client.defaultPassword

			uDefaulted := len(client.defaultUsername) > 0
			pDefaulted := len(client.defaultPassword) > 0

			if util.IsTerminal(client.reader) && !(uDefaulted && pDefaulted) {
				fmt.Printf("Authenticate for \"%v\"\n", realm)
				if !uDefaulted {
					username = util.PromptForString(client.reader, "Username: ")
				}
				if !pDefaulted {
					password = util.PromptForPasswordString(client.reader, "Password: ")
				}
			}

			client.delegate.Transport = kclient.NewBasicAuthRoundTripper(username, password, client.defaultRoundTripper)
			return client.Do(resp.Request)
		}
	}
	return resp, err
}

func isBasicAuthChallenge(resp *http.Response) (bool, string) {
	for currHeader, headerValue := range resp.Header {
		if strings.EqualFold(currHeader, "WWW-Authenticate") {
			for _, currAuthorizeHeader := range headerValue {
				if matches := basicAuthRegex.FindAllStringSubmatch(currAuthorizeHeader, 1); matches != nil {
					return true, matches[0][1]
				}
			}
		}
	}

	return false, ""
}
