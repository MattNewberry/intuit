/*
Go client for Intuit's Customer Account Data API
*/
package intuit

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/MattNewberry/oauth"
	"net/http"
	"time"
)

const (
	InstitutionXMLNS = "http://schema.intuit.com/platform/fdatafeed/institutionlogin/v1"
	ChallengeXMLNS   = "http://schema.intuit.com/platform/fdatafeed/challenge/v1"
)

type InstitutionLogin struct {
	XMLName     xml.Name    `xml:"InstitutionLogin"`
	XMLNS       string      `xml:"xmlns,attr"`
	Credentials Credentials `xml:"credentials,omitempty"`
}

type InstitutionLoginMFA struct {
	XMLName            xml.Name           `xml:"InstitutionLogin"`
	XMLNS              string             `xml:"xmlns,attr"`
	ChallengeResponses ChallengeResponses `xml:"challengeResponses"`
}

type Credentials struct {
	Credentials []Credential
}

type Credential struct {
	XMLName xml.Name `xml:"credential"`
	Name    string   `xml:"name"`
	Value   string   `xml:"value"`
}

type ChallengeResponses struct {
	ChallengeResponses []ChallengeResponse
}

type Challenge struct {
	Question string
	Choices  []Choice
}

type ChallengeResponse struct {
	XMLName xml.Name    `xml:"v11:response"`
	XMLNS   string      `xml:"xmlns:v11,attr"`
	Answer  interface{} `xml:",innerxml"`
}

type Choice struct {
	Value interface{}
	Text  string
}

type ChallengeSession struct {
	InstitutionId string
	SessionId     string
	NodeId        string
	Challenges    []Challenge
	Answers       []interface{}
}

type Configuration struct {
	CustomerID          string
	OAuthConsumerKey    string
	OAuthConsumerSecret string
	oAuthToken          *oauth.AccessToken
	SamlProviderID      string
	CertificatePath     string
}

const (
	BaseURL = "https://financialdatafeed.platform.intuit.com/v1/"
	GET     = "GET"
	POST    = "POST"
	DELETE  = "DELETE"
)

var SessionConfiguration *Configuration

/*
Configure the client for access to your application.
*/
func Configure(configuration *Configuration) {
	SessionConfiguration = configuration
}

/*
Set the customer ID for the current session.
*/
func Scope(id string) {
	if SessionConfiguration == nil {
		SessionConfiguration = &Configuration{}
	}

	SessionConfiguration.CustomerID = id
}

/*
Discover new accounts for a customer, returning an MFA response if applicable.

In practice, the most efficient workflow is to cache the Institutions list and pass the username and password keys to this method. Without doing so, fetching the instituion's details will be required.
*/
func DiscoverAndAddAccounts(institutionID string, username string, password string, usernameKey string, passwordKey string) (accounts []interface{}, challengeSession *ChallengeSession, err error) {
	userCredential := Credential{Name: usernameKey, Value: username}
	passwordCredential := Credential{Name: passwordKey, Value: password}
	credentials := Credentials{Credentials: []Credential{userCredential, passwordCredential}}

	payload := &InstitutionLogin{Credentials: credentials, XMLNS: InstitutionXMLNS}
	data, err := post(fmt.Sprintf("institutions/%v/logins", institutionID), payload, nil, nil)

	if err == nil {
		// Success
		accounts = data.(map[string]interface{})["accounts"].([]interface{})
	} else if data != nil {
		// MFA
		challengeData := data.(map[string]interface{})
		httpError := err.(oauth.HTTPExecuteError)
		headers := httpError.ResponseHeaders

		challengeSession = &ChallengeSession{InstitutionId: institutionID}
		challengeSession.SessionId = headers.Get("Challengesessionid")
		challengeSession.NodeId = headers.Get("Challengenodeid")
		challengeSession.Challenges = make([]Challenge, 0)
		challenges := challengeData["challenge"].([]interface{})

		for _, c := range challenges {
			chal := c.(map[string]interface{})

			for _, v := range chal {
				vData := v.([]interface{})
				challenge := Challenge{}

				for i, val := range vData {
					if i == 0 {
						challenge.Question = val.(string)
						challenge.Choices = make([]Choice, 0)
					} else {
						cData := val.(map[string]interface{})
						choice := Choice{Value: cData["val"].(string), Text: cData["text"].(string)}
						challenge.Choices = append(challenge.Choices, choice)
					}
				}

				challengeSession.Challenges = append(challengeSession.Challenges, challenge)
			}
		}
	}
	return
}

/*
When prompted with an MFA challenge, reply with an answer to the challenges.
*/
func RespondToChallenge(session *ChallengeSession) (data interface{}, err error) {
	responses := make([]ChallengeResponse, len(session.Challenges))
	for i, r := range session.Answers {
		responses[i] = ChallengeResponse{Answer: r, XMLNS: ChallengeXMLNS}
	}

	response := ChallengeResponses{ChallengeResponses: responses}
	payload := &InstitutionLoginMFA{ChallengeResponses: response, XMLNS: InstitutionXMLNS}
	headers := map[string][]string{
		"challengeNodeId":    []string{session.NodeId},
		"challengeSessionId": []string{session.SessionId},
	}

	data, err = post(fmt.Sprintf("institutions/%v/logins", session.InstitutionId), payload, nil, headers)
	return
}

/*
Return all accounts stored for the scoped customer.
*/
func Accounts() ([]interface{}, error) {
	res, err := get("accounts", nil)

	data := res.(map[string]interface{})
	return data["accounts"].([]interface{}), err
}

/*
Return a specific account for the scoped customer, given it's ID.
*/
func Account(accountID string) (map[string]interface{}, error) {
	res, err := get(fmt.Sprintf("accounts/%s", accountID), nil)

	data := res.(map[string]interface{})
	account := data["accounts"].([]interface{})
	return account[0].(map[string]interface{}), err
}

/*
Get all transactions for an account, filtered by the given start and end times.
*/
func Transactions(accountID string, start time.Time, end time.Time) (map[string]interface{}, error) {

	params := make(map[string]string)
	const timeFormat = "2006-01-02"
	params["txnStartDate"] = start.Format(timeFormat)
	params["tnxEndDate"] = end.Format(timeFormat)
	res, err := get(fmt.Sprintf("accounts/%s/transactions", accountID), params)

	data := res.(map[string]interface{})
	return data, err
}

/*
Retrieve all known institutions.

Given the volume of institutions supported, this call can be very time consuming.
*/
func Institutions() ([]interface{}, error) {
	res, err := get("institutions", nil)

	data := res.(map[string]interface{})
	all := data["institution"].([]interface{})
	return all, err
}

/*
Retrieve an institution's detailed information.
*/
func Institution(institutionID string) (data map[string]interface{}, err error) {
	res, err := get(fmt.Sprintf("institutions/%s", institutionID), nil)

	if res != nil {
		data = res.(map[string]interface{})
	}
	return
}

/*
Delete the scoped customer and all related accounts.
*/
func DeleteCustomer() error {
	_, err := request(DELETE, "customers", "", nil, nil)
	return err
}

/*
Delete an account for the scoped customer.
*/
func DeleteAccount(accountID string) error {
	_, err := request(DELETE, "accounts/"+accountID, "", nil, nil)
	return err
}

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
