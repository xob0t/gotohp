package backend

import (
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

func NewHTTPClientWithProxy(proxyURLStr string) (*http.Client, error) {
	// Create the base transport with default values
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = false

	// Configure proxy if provided
	if proxyURLStr != "" {
		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	// Create retryable client with proper configuration
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second   // Start with 1 second
	retryClient.RetryWaitMax = 30 * time.Second  // Maximum wait time
	retryClient.HTTPClient.Transport = transport // Set transport here

	// Important: Configure the retry policy to retry on connection errors
	retryClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy

	return retryClient.StandardClient(), nil
}
