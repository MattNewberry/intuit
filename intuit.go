/*
Go client for Intuit's Customer Account Data API
*/
package intuit

import (
	"encoding/xml"
	"fmt"
	"github.com/MattNewberry/oauth"
	"time"
)

const (
	InstitutionXMLNS = "http://schema.intuit.com/platform/fdatafeed/institutionlogin/v1"
	ChallengeXMLNS   = "http://schema.intuit.com/platform/fdatafeed/challenge/v1"

	BaseURL = "https://financialdatafeed.platform.intuit.com/v1/"
	GET     = "GET"
	POST    = "POST"
	DELETE  = "DELETE"
	PUT     = "PUT"

	updateLoginType = 1 + iota
	discoverAndAddType
)

var SessionConfiguration *Configuration

type challengeContextType int

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
	LoginId       string
	SessionId     string
	NodeId        string
	Challenges    []Challenge
	Answers       []interface{}
	contextType   challengeContextType
}

type Configuration struct {
	CustomerId          string
	OAuthConsumerKey    string
	OAuthConsumerSecret string
	oAuthToken          *oauth.AccessToken
	SamlProviderId      string
	CertificatePath     string
}

/*
Configure the client for access to your application.
*/
func Configure(configuration *Configuration) {
	SessionConfiguration = configuration
}

/*
Set the customer Id for the current session.
*/
func Scope(id string) {
	if SessionConfiguration == nil {
		SessionConfiguration = &Configuration{}
	}

	SessionConfiguration.CustomerId = id
}

/*
Discover new accounts for a customer, returning an MFA response if applicable.

In practice, the most efficient workflow is to cache the Institutions list and pass the username and password keys to this method. Without doing so, fetching the instituion's details will be required.
*/
func DiscoverAndAddAccounts(institutionId string, username string, password string, usernameKey string, passwordKey string) (accounts []interface{}, challengeSession *ChallengeSession, err error) {
	userCredential := Credential{Name: usernameKey, Value: username}
	passwordCredential := Credential{Name: passwordKey, Value: password}
	credentials := Credentials{Credentials: []Credential{userCredential, passwordCredential}}

	payload := &InstitutionLogin{Credentials: credentials, XMLNS: InstitutionXMLNS}
	data, err := post(fmt.Sprintf("institutions/%v/logins", institutionId), payload, nil, nil)

	if err == nil {
		// Success
		accounts = data.(map[string]interface{})["accounts"].([]interface{})
	} else if data != nil {
		challengeSession = parseChallengeSession(discoverAndAddType, data, err)
		challengeSession.InstitutionId = institutionId
	}

	return
}

/*
Update login information for an account, returning an MFA response if applicable.
*/
func UpdateLoginAccount(loginId string, username string, password string, usernameKey string, passwordKey string) (accounts []interface{}, challengeSession *ChallengeSession, err error) {
	userCredential := Credential{Name: usernameKey, Value: username}
	passwordCredential := Credential{Name: passwordKey, Value: password}
	credentials := Credentials{Credentials: []Credential{userCredential, passwordCredential}}

	payload := &InstitutionLogin{Credentials: credentials, XMLNS: InstitutionXMLNS}
	data, err := put(fmt.Sprintf("logins/%v?refresh=true", loginId), payload, nil, nil)

	if err == nil {
		// Success
		accounts = data.(map[string]interface{})["accounts"].([]interface{})
	} else if data != nil {
		challengeSession = parseChallengeSession(updateLoginType, data, err)
		challengeSession.LoginId = loginId
	}

	return
}

/*
Return all accounts stored for the scoped customer.
*/
func LoginAccounts(loginId string) ([]interface{}, error) {
	res, err := get(fmt.Sprintf("logins/%v/accounts", loginId), nil)

	data := res.(map[string]interface{})
	return data["accounts"].([]interface{}), err
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

	switch session.contextType {
	case discoverAndAddType:
		data, err = post(fmt.Sprintf("institutions/%v/logins", session.InstitutionId), payload, nil, headers)
	case updateLoginType:
		data, err = put(fmt.Sprintf("logins/%v", session.LoginId), payload, nil, headers)
	}

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
Return a specific account for the scoped customer, given it's Id.
*/
func Account(accountId string) (map[string]interface{}, error) {
	res, err := get(fmt.Sprintf("accounts/%s", accountId), nil)

	data := res.(map[string]interface{})
	account := data["accounts"].([]interface{})
	return account[0].(map[string]interface{}), err
}

/*
Get all transactions for an account, filtered by the given start and end times.
*/
func Transactions(accountId string, start time.Time, end time.Time) (map[string]interface{}, error) {

	params := make(map[string]string)
	const timeFormat = "2006-01-02"
	params["txnStartDate"] = start.Format(timeFormat)
	params["tnxEndDate"] = end.Format(timeFormat)
	res, err := get(fmt.Sprintf("accounts/%s/transactions", accountId), params)

	var data map[string]interface{}
	if err == nil {
		data = res.(map[string]interface{})
	}

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
func Institution(institutionId string) (data map[string]interface{}, err error) {
	res, err := get(fmt.Sprintf("institutions/%s", institutionId), nil)

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
func DeleteAccount(accountId string) error {
	_, err := request(DELETE, "accounts/"+accountId, "", nil, nil)
	return err
}

func parseChallengeSession(contextType challengeContextType, data interface{}, err error) *ChallengeSession {
	challengeData := data.(map[string]interface{})
	httpError := err.(oauth.HTTPExecuteError)
	headers := httpError.ResponseHeaders

	var challengeSession = &ChallengeSession{contextType: contextType}
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

	return challengeSession
}
