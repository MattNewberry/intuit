#Intuit

Go client for Intuit's Customer Account Data API. 

Information about the service can be found at [Intuit](https://developer.intuit.com/docs/0020_customeraccountdata).

## Documentation
Full documentation can be found on [Godoc.org](http://godoc.org/github.com/MattNewberry/intuit).

## Quick Start
To begin, add your public key used to create your application to your project directory. Next, set the configuration information unique to your application.

````
config := &intuit.Configuration{
	CertificatePath:     "cert.key",
	OAuthConsumerKey:    "a182b398wdhjwbahs",
	OAuthConsumerSecret: "jwiu38ufn2f82nfn1fn",
	SamlProviderID:      "app.1.cc.dev-intuit.ipp.prod",
}
intuit.Configure(config)
intuit.Scope("testing")

accounts, err := intuit.Accounts()
````
