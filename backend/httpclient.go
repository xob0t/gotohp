package backend

import (
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

func NewHTTPClientWithProxy(proxyURLStr string) (*http.Client, error) {
	// Create the base transport with default values
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = false

	// Configure connection pooling for concurrent uploads
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.MaxConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second

	// Configure proxy if provided
	if proxyURLStr != "" {
		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // No timeout for large uploads - context handles cancellation
	}

	return client, nil
}

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// DefaultRetryConfig returns sensible defaults for retries
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
	}
}

// ShouldRetry determines if an HTTP response warrants a retry
func ShouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true // Network errors should be retried
	}
	if resp == nil {
		return true
	}
	// Retry on 5xx server errors and 429 (rate limit)
	return resp.StatusCode >= 500 || resp.StatusCode == 429
}

// CalculateBackoff returns the delay for a given attempt (exponential backoff with jitter)
func CalculateBackoff(attempt int, config RetryConfig) time.Duration {
	delay := config.InitialDelay * time.Duration(1<<uint(attempt))
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}
	// Add up to 10% jitter to prevent thundering herd
	jitter := time.Duration(rand.Int63n(int64(delay / 10)))
	return delay + jitter
}
