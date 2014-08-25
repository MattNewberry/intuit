package intuit

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/MattNewberry/oauth"
	"github.com/nu7hatch/gouuid"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
)

type assertion struct {
	IssuerID    string
	UserID      string
	ReferenceID string
	TimeNow     string
	TimeBefore  string
	TimeAfter   string
	Signature   string
}

type signedInfo struct {
	ReferenceID string
	Digest      string
}

type signature struct {
	SignatureValue string
	SignedInfo     string
}

func MakeSamlAssertion() (*oauth.AccessToken, error) {
	a := &assertion{}
	a.IssuerID = SessionConfiguration.SamlProviderID
	a.UserID = SessionConfiguration.CustomerID
	a.ReferenceID = a.newUUID()

	t := time.Now()
	a.TimeNow = a.formatTimeFromDuration(t, 0)
	a.TimeBefore = a.formatTimeFromDuration(t, -5*time.Minute)
	a.TimeAfter = a.formatTimeFromDuration(t, 10*time.Minute)

	si := signedInfoFromAssertion(a)

	s := &signature{}
	s.SignatureValue = si.SignatureValue(SessionConfiguration.CertificatePath)
	s.SignedInfo = si.String()

	signature := s.String()
	a.Signature = signature

	payload := base64.URLEncoding.EncodeToString([]byte(a.String()))

	values := make(url.Values)
	values.Set("saml_assertion", payload)
	values.Set("oauth_consumer_key", SessionConfiguration.OAuthConsumerKey)
	resp, err := http.PostForm("https://oauth.intuit.com/oauth/v1/get_access_token_by_saml", values)

	tokens := &oauth.AccessToken{}
	if err != nil || resp.StatusCode != 200 {
		db, _ := url.QueryUnescape(resp.Header.Get("Www-Authenticate"))
		msg := fmt.Sprintf("%s %s", resp.Status, db)
		err = errors.New(msg)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		bValues, _ := url.ParseQuery(string(body))
		tokens.Token = bValues.Get("oauth_token")
		tokens.Secret = bValues.Get("oauth_token_secret")
	}

	return tokens, err
}

func (a *assertion) String() string {
	return parseTemplate("saml_assertion", a)
}

func (s *signature) String() string {
	return parseTemplate("saml_signature", s)
}

func (s *signedInfo) String() string {
	return parseTemplate("saml_signed", s)
}

func parseTemplate(file string, data interface{}) string {
	t, _ := template.ParseFiles("templates/" + file + ".xml")

	var buf bytes.Buffer
	t.Execute(&buf, data)
	return buf.String()
}

func sha1Encode(a string) string {
	h := sha1.New()
	h.Write([]byte(a))
	return string(h.Sum(nil))
}

func (a *assertion) formatTimeFromDuration(t time.Time, d time.Duration) string {
	const layout = "2006-01-02T15:04:05"
	return fmt.Sprintf("%s.000Z", t.Add(d).UTC().Format(layout))
}

func (a *assertion) newUUID() string {
	uuid, _ := uuid.NewV4()
	return fmt.Sprintf("_%s", strings.Replace(uuid.String(), "-", "", -1))
}

func signedInfoFromAssertion(a *assertion) *signedInfo {
	s := &signedInfo{}
	s.ReferenceID = a.ReferenceID

	sha := sha1Encode(a.String())
	s.Digest = base64.StdEncoding.EncodeToString([]byte(sha))

	return s
}

func (s *signedInfo) SignatureValue(keyPath string) string {
	pkey, err := ioutil.ReadFile(keyPath)
	if err != nil {
		panic(err)
	}

	block, _ := pem.Decode(pkey)
	if block == nil {
		panic(fmt.Sprintf("bad key data: %s", "not PEM-encoded"))
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(fmt.Sprintf("bad private key: %s", err))
	}

	signedString := s.String()
	digest := []byte(sha1Encode(signedString))

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA1, digest)
	if err != nil {
		panic(fmt.Sprintf("rsa.SignPKCS1v15 error: %v\n", err))
	}

	return base64.StdEncoding.EncodeToString([]byte(signature))
}
