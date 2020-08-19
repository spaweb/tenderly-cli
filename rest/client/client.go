package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/tenderly/tenderly-cli/config"
	"github.com/tenderly/tenderly-cli/rest/payloads"
	"github.com/tenderly/tenderly-cli/userError"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func Request(method, path string, body []byte) io.Reader {
	apiBase := "https://api.tenderly.co"
	if alternativeApiBase := config.MaybeGetString("api_base"); len(alternativeApiBase) != 0 {
		apiBase = alternativeApiBase
	}

	requestUrl := fmt.Sprintf("%s/%s", apiBase, path)
	req, err := http.NewRequest(
		method,
		requestUrl,
		bytes.NewReader(body),
	)
	if err != nil {
		userError.LogErrorf("failed creating request: %s", userError.NewUserError(
			err,
			"Failed creating request. Please try again.",
		))
		os.Exit(1)
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: false}

	if key := config.GetAccessKey(); key != "" {
		// set access key
		req.Header.Add("x-access-key", key)
	} else if token := config.GetToken(); token != "" {
		// set auth token
		req.Header.Add("Authorization", "Bearer "+token)

		urlPath := fmt.Sprintf("api/v1/account/%s/token", config.GetAccountId())
		if requestUrl != fmt.Sprintf("%s/%s", apiBase, urlPath) {
			var request payloads.GenerateAccessTokenRequest
			request.Name = "CLI access token"

			body, err := json.Marshal(request)
			if err != nil {
				logrus.Debug("failed to marshall generate access token request", logrus.Fields{
					"url_path":   urlPath,
					"account_id": config.GetAccountId(),
				})
			} else {
				reader := Request(
					"POST",
					urlPath,
					body,
				)

				var tokenResp payloads.TokenResponse
				err := json.NewDecoder(reader).Decode(&tokenResp)

				if err != nil || tokenResp.Error != nil {
					logrus.Debug("failed creating token")
					return nil
				}

				config.SetGlobalConfig(config.AccessKey, tokenResp.Token)
				config.SetGlobalConfig(config.AccessKeyId, tokenResp.ID)

				//@TODO(filip): remove this once we
				err = config.WriteGlobalConfig()
				if err != nil {
					userError.LogErrorf(
						"write global config: %s",
						userError.NewUserError(err, "Couldn't write global config file"),
					)

					return nil
				}

				req.Header.Add("x-access-key", tokenResp.Token)
				req.Header.Del("Authorization")
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"request_url":  requestUrl,
		"request_body": string(body),
	}).Debug("Making request")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		userError.LogErrorf("failed making request: %s", userError.NewUserError(
			err,
			"Failed making request. Please try again.",
		))
		os.Exit(1)
	}

	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		userError.LogErrorf("failed reading response body: %s", userError.NewUserError(
			err,
			"Failed reading response body. Please try again.",
		))
		os.Exit(1)
	}

	logrus.WithField("response_body", data).Debug("Got response with body")

	return bytes.NewReader(data)
}
