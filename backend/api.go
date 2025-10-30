package backend

import (
	"app/generated"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

type Api struct {
	androidAPIVersion int64
	model             string
	make              string
	clientVersionCode int64
	userAgent         string
	language          string
	authData          string
	client            *http.Client
	authResponseCache map[string]string
}

type AuthResponse struct {
	Expiry string
	Auth   string
}

func NewApi() (*Api, error) {
	selectedEmail := AppConfig.Selected
	if len(selectedEmail) == 0 {
		return nil, fmt.Errorf("no account is selected")
	}
	credentials := ""
	language := ""
	for _, c := range AppConfig.Credentials {
		params, err := url.ParseQuery(c)
		if err != nil {
			continue
		}
		if params.Get("Email") == selectedEmail {
			credentials = c
			language = params.Get("lang")
		}
	}

	if len(credentials) == 0 {
		return nil, fmt.Errorf("no credentials with matching selcted email found")
	}

	client, err := NewHTTPClientWithProxy(AppConfig.Proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	api := &Api{
		androidAPIVersion: 28,
		model:             "Pixel XL",
		make:              "Google",
		clientVersionCode: 49029607,
		language:          language,
		authData:          strings.TrimSpace(credentials),
		client:            client,
		authResponseCache: map[string]string{
			"Expiry": "0",
			"Auth":   "",
		},
	}

	api.userAgent = fmt.Sprintf(
		"com.google.android.apps.photos/%d (Linux; U; Android 9; %s; %s; Build/PQ2A.190205.001; Cronet/127.0.6510.5) (gzip)",
		api.clientVersionCode,
		api.language,
		api.model,
	)

	return api, nil
}

func (a *Api) BearerToken() (string, error) {
	expiryStr := a.authResponseCache["Expiry"]
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expiry time: %w", err)
	}

	if expiry <= time.Now().Unix() {
		resp, err := a.getAuthToken()
		if err != nil {
			return "", fmt.Errorf("failed to get auth token: %w", err)
		}
		a.authResponseCache = resp
	}

	if token, ok := a.authResponseCache["Auth"]; ok && token != "" {
		return token, nil
	}

	return "", errors.New("auth response does not contain bearer token")
}

func (a *Api) getAuthToken() (map[string]string, error) {
	authDataValues, err := url.ParseQuery(a.authData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth data: %w", err)
	}

	authRequestData := url.Values{
		"androidId":                    {authDataValues.Get("androidId")},
		"app":                          {"com.google.android.apps.photos"},
		"client_sig":                   {authDataValues.Get("client_sig")},
		"callerPkg":                    {"com.google.android.apps.photos"},
		"callerSig":                    {authDataValues.Get("callerSig")},
		"device_country":               {authDataValues.Get("device_country")},
		"Email":                        {authDataValues.Get("Email")},
		"google_play_services_version": {authDataValues.Get("google_play_services_version")},
		"lang":                         {authDataValues.Get("lang")},
		"oauth2_foreground":            {authDataValues.Get("oauth2_foreground")},
		"sdk_version":                  {authDataValues.Get("sdk_version")},
		"service":                      {authDataValues.Get("service")},
		"Token":                        {authDataValues.Get("Token")},
	}

	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"app":             "com.google.android.apps.photos",
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded",
		"device":          authRequestData.Get("androidId"),
		"User-Agent":      "GoogleAuth/1.4 (Pixel XL PQ2A.190205.001); gzip",
	}

	req, err := http.NewRequest(
		"POST",
		"https://android.googleapis.com/auth",
		strings.NewReader(authRequestData.Encode()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request failed after retries: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return make(map[string]string), fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Handle gzip encoding if present
	var reader io.Reader
	reader, err = gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.(*gzip.Reader).Close()

	// Parse the response body
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the key=value response format
	parsedAuthResponse := make(map[string]string)
	for _, line := range strings.Split(string(bodyBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			parsedAuthResponse[parts[0]] = parts[1]
		}
	}

	// Validate we got the required fields
	if parsedAuthResponse["Auth"] == "" {
		return nil, errors.New("auth response missing Auth token")
	}
	if parsedAuthResponse["Expiry"] == "" {
		return nil, errors.New("auth response missing Expiry")
	}

	return parsedAuthResponse, nil
}

// Obtain a file upload token from the Google Photos API.
func (a *Api) GetUploadToken(shaHashB64 string, fileSize int64) (string, error) {
	// Create the protobuf message
	protoBody := generated.GetUploadToken{
		F1:            2,
		F2:            2,
		F3:            1,
		F4:            3,
		FileSizeBytes: fileSize,
	}

	// Serialize the protobuf message
	serializedData, err := proto.Marshal(&protoBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Get the bearer token
	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Accept-Encoding":         "gzip",
		"Accept-Language":         a.language,
		"Content-Type":            "application/x-protobuf",
		"User-Agent":              a.userAgent,
		"Authorization":           "Bearer " + bearerToken,
		"X-Goog-Hash":             "sha1=" + shaHashB64,
		"X-Upload-Content-Length": strconv.Itoa(int(fileSize)),
	}

	// Create the request
	req, err := http.NewRequest(
		"POST",
		"https://photos.googleapis.com/data/upload/uploadmedia/interactive",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make the request
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Get the upload token from headers
	uploadToken := resp.Header.Get("X-GUploader-UploadID")
	if uploadToken == "" {
		return "", errors.New("response missing X-GUploader-UploadID header")
	}

	return uploadToken, nil
}

// Check library for existing files with the hash
func (a *Api) FindRemoteMediaByHash(shaHash []byte) (string, error) {
	// Create the protobuf message

	// Create and initialize the protobuf message with all required nested structures
	protoBody := generated.HashCheck{
		Field1: &generated.HashCheckField1Type{
			Field1: &generated.HashCheckField1TypeField1Type{
				Sha1Hash: shaHash,
			},
			Field2: &generated.HashCheckField1TypeField2Type{},
		},
	}

	// Serialize the protobuf message
	serializedData, err := proto.Marshal(&protoBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Get the bearer token
	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"Accept-Language": a.language,
		"Content-Type":    "application/x-protobuf",
		"User-Agent":      a.userAgent,
		"Authorization":   "Bearer " + bearerToken,
	}

	// Create the request
	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/5084965799730810217",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make the request
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader
	reader, err = gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.(*gzip.Reader).Close()

	// Parse the response body
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var pbResp generated.RemoteMatches
	if err := proto.Unmarshal(bodyBytes, &pbResp); err != nil {
		log.Fatalf("Failed to unmarshal protobuf: %v", err)
	}

	mediaKey := pbResp.GetMediaKey()

	return mediaKey, nil
}

func (a *Api) UploadFile(ctx context.Context, filePath string, uploadToken string) (*generated.CommitToken, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	uploadURL := "https://photos.googleapis.com/data/upload/uploadmedia/interactive?upload_id=" + uploadToken

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Important: Don't set ContentLength to enable chunked transfer encoding
	req.ContentLength = -1

	bearerToken, err := a.BearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"Accept-Language": a.language,
		"User-Agent":      a.userAgent,
		"Authorization":   "Bearer " + bearerToken,
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pbResp generated.CommitToken
	if err := proto.Unmarshal(bodyBytes, &pbResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	return &pbResp, nil
}

// CommitUpload commits the upload to Google Photos
func (a *Api) CommitUpload(
	uploadResponseDecoded *generated.CommitToken,
	fileName string,
	sha1Hash []byte,
	uploadTimestamp int64,
) (string, error) {
	if uploadTimestamp == 0 {
		uploadTimestamp = time.Now().Unix()
	}

	var qualityVal int64 = 3
	if AppConfig.Saver {
		qualityVal = 1
		a.model = "Pixel 2"
	}

	if AppConfig.UseQuota {
		a.model = "Pixel 8"
	}

	unknownInt := int64(46000000)

	// Create the protobuf message
	protoBody := generated.CommitUpload{
		Field1: &generated.CommitUploadField1Type{
			Field1: &generated.CommitUploadField1TypeField1Type{
				Field1: uploadResponseDecoded.Field1,
				Field2: uploadResponseDecoded.Field2,
			},
			FileName: fileName,
			Sha1Hash: sha1Hash,
			Field4: &generated.CommitUploadField1TypeField4Type{
				FileLastModifiedTimestamp: uploadTimestamp,
				Field2:                    unknownInt,
			},
			Quality: qualityVal,
			Field10: 1,
		},
		Field2: &generated.CommitUploadField2Type{
			Model:             a.model,
			Make:              a.make,
			AndroidApiVersion: a.androidAPIVersion,
		},
		Field3: []byte{1, 3},
	}

	// Serialize the protobuf message
	serializedData, err := proto.Marshal(&protoBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Get the bearer token
	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	// Prepare headers
	headers := map[string]string{
		"accept-Encoding":          "gzip",
		"accept-Language":          a.language,
		"content-Type":             "application/x-protobuf",
		"user-Agent":               a.userAgent,
		"authorization":            "Bearer " + bearerToken,
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	// Create the request
	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/16538846908252377752",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make the request
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Handle gzip response if needed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.(*gzip.Reader).Close()
	}

	// Parse the response body
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var pbResp generated.CommitUploadResponse
	if err := proto.Unmarshal(bodyBytes, &pbResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	// Get media key from response
	if pbResp.GetField1() == nil || pbResp.GetField1().GetField3() == nil {
		return "", fmt.Errorf("upload rejected by API: invalid response structure")
	}

	mediaKey := pbResp.GetField1().GetField3().GetMediaKey()
	if mediaKey == "" {
		return "", fmt.Errorf("upload rejected by API: no media key returned")
	}

	return mediaKey, nil
}
