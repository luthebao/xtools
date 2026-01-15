package twitter

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"xtools/internal/domain"
)

// generateOAuthHeader creates an OAuth 1.0a Authorization header
func generateOAuthHeader(creds domain.APICredentials, method, reqURL string) string {
	// Parse URL to separate base URL and query params
	parsedURL, _ := url.Parse(reqURL)
	baseURL := parsedURL.Scheme + "://" + parsedURL.Host + parsedURL.Path

	// Generate OAuth parameters
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce()

	oauthParams := map[string]string{
		"oauth_consumer_key":     creds.APIKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            creds.AccessToken,
		"oauth_version":          "1.0",
	}

	// Collect all parameters (OAuth + query params)
	allParams := make(map[string]string)
	for k, v := range oauthParams {
		allParams[k] = v
	}
	for k, v := range parsedURL.Query() {
		if len(v) > 0 {
			allParams[k] = v[0]
		}
	}

	// Create signature base string
	paramString := createOAuthParamString(allParams)
	signatureBase := strings.ToUpper(method) + "&" +
		url.QueryEscape(baseURL) + "&" +
		url.QueryEscape(paramString)

	// Create signing key
	signingKey := url.QueryEscape(creds.APISecret) + "&" +
		url.QueryEscape(creds.AccessSecret)

	// Generate signature
	signature := generateHMACSHA1Signature(signatureBase, signingKey)
	oauthParams["oauth_signature"] = signature

	// Build Authorization header
	var headerParts []string
	for k, v := range oauthParams {
		headerParts = append(headerParts, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
	}
	sort.Strings(headerParts)

	return "OAuth " + strings.Join(headerParts, ", ")
}

// hasOAuthCredentials checks if OAuth 1.0a credentials are available
func hasOAuthCredentials(creds domain.APICredentials) bool {
	return creds.APIKey != "" &&
		creds.APISecret != "" &&
		creds.AccessToken != "" &&
		creds.AccessSecret != ""
}

// generateNonce creates a random nonce for OAuth
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// createOAuthParamString creates a sorted parameter string for OAuth signature
func createOAuthParamString(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	return strings.Join(parts, "&")
}

// generateHMACSHA1Signature creates HMAC-SHA1 signature for OAuth
func generateHMACSHA1Signature(base, key string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
