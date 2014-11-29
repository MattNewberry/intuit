package intuit

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/MattNewberry/oauth"
	"net/http"
)

func post(endpoint string, body interface{}, params map[string]string, headers map[string][]string) (interface{}, error) {
	return request(POST, endpoint, body, params, headers)
}

func get(endpoint string, params map[string]string) (interface{}, error) {
	return request(GET, endpoint, "", params, nil)
}

func request(method string, endpoint string, body interface{}, params map[string]string, headers map[string][]string) (data interface{}, err error) {
	if SessionConfiguration.oAuthToken == nil {
		SessionConfiguration.oAuthToken, err = MakeSamlAssertion()

		if err != nil {
			return
		}
	}

	c := oauth.NewConsumer(
		SessionConfiguration.OAuthConsumerKey,
		SessionConfiguration.OAuthConsumerSecret,
		oauth.ServiceProvider{})
	c.AdditionalHeaders = map[string][]string{
		"Accept":       []string{"application/json"},
		"Content-Type": []string{"application/xml"},
	}

	for k, v := range headers {
		c.AdditionalHeaders[k] = v
	}

	url := fmt.Sprintf("%s%s", BaseURL, endpoint)
	var res *http.Response

	if method == GET {
		res, err = c.Get(url, params, SessionConfiguration.oAuthToken)
	} else if method == POST {
		payload, _ := xml.MarshalIndent(body, "  ", "    ")
		res, err = c.Post(url, string(payload), params, SessionConfiguration.oAuthToken)
	} else if method == DELETE {
		res, err = c.Delete(url, params, SessionConfiguration.oAuthToken)
	}

	if err == nil {
		d := json.NewDecoder(res.Body)
		d.UseNumber()
		err = d.Decode(&data)
	} else {
		httpError := err.(oauth.HTTPExecuteError)
		json.Unmarshal(httpError.ResponseBodyBytes, &data)
	}

	return data, err
}
