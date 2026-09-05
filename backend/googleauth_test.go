package backend

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGoogleAuthenticationRejectsRedirects(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, destination := range []string{"same-origin", "cross-origin", "http-downgrade"} {
			for _, validation := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/validation=%t", status, destination, validation), func(t *testing.T) {
					const token = "synthetic-redirect-test-token"
					var redirected atomic.Int32
					var initial atomic.Int32
					redirectTarget := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						redirected.Add(1)
						w.WriteHeader(http.StatusBadRequest)
					}))
					if destination == "http-downgrade" {
						redirectTarget.Start()
					} else {
						redirectTarget.StartTLS()
					}
					defer redirectTarget.Close()
					location := redirectTarget.URL + "/redirected?token=" + token
					source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/redirected" {
							redirected.Add(1)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						initial.Add(1)
						if r.Method != http.MethodPost {
							t.Errorf("method = %s, want POST", r.Method)
						}
						if err := r.ParseForm(); err != nil || r.Form.Get("Token") != token {
							t.Error("initial request did not contain the expected token form")
						}
						http.Redirect(w, r, location, status)
					}))
					defer source.Close()
					if destination == "same-origin" {
						location = source.URL + "/redirected?token=" + token
					}

					// Route only the fixed validation endpoint to the local TLS server.
					// Keep real redirect targets reachable so an accidental follow is observed.
					transport := http.DefaultTransport.(*http.Transport).Clone()
					transport.Proxy = nil
					transport.TLSClientConfig = source.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
					transport.TLSClientConfig.InsecureSkipVerify = false
					transport.TLSClientConfig.ServerName = "127.0.0.1"
					transport.TLSClientConfig.RootCAs = x509.NewCertPool()
					transport.TLSClientConfig.RootCAs.AddCert(source.Certificate())
					if destination != "http-downgrade" {
						transport.TLSClientConfig.RootCAs.AddCert(redirectTarget.Certificate())
					}
					transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
						if address == "android.googleapis.com:443" {
							address = source.Listener.Addr().String()
						}
						return (&net.Dialer{}).DialContext(ctx, network, address)
					}
					previousTransport := http.DefaultTransport
					http.DefaultTransport = transport
					defer func() { http.DefaultTransport = previousTransport }()

					var err error
					if validation {
						credential := buildGooglePhotosCredential("person@example.com", token, "0123456789abcdef")
						err = validateGooglePhotosCredential(credential, "")
					} else {
						client, clientErr := newGoogleAuthHTTPClient("")
						if clientErr != nil {
							t.Fatal(clientErr)
						}
						defer client.CloseIdleConnections()
						_, err = exchangeEmbeddedSetupToken(context.Background(), client, source.URL, token, "0123456789abcdef")
					}
					if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
						t.Errorf("expected rejection with HTTP status %d", status)
					}
					if err != nil && strings.Contains(err.Error(), token) {
						t.Error("error exposed token from redirect Location")
					}
					if initial.Load() != 1 || redirected.Load() != 0 {
						t.Errorf("initial requests = %d, redirected requests = %d; want 1, 0", initial.Load(), redirected.Load())
					}
				})
			}
		}
	}
}

func TestGoogleAuthHTTPClientHandlesNegotiatedHTTP2(t *testing.T) {
	const masterToken = "test-master-token"

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.ProtoMajor != 2 {
			t.Errorf("protocol = %s, want HTTP/2", request.Proto)
		}
		_, _ = fmt.Fprintf(writer, "Token=%s\nEmail=person@example.com\n", masterToken)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	client, err := newGoogleAuthHTTPClient("")
	if err != nil {
		t.Fatalf("create Google auth client: %v", err)
	}
	transport := client.Transport.(*http.Transport)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport.TLSClientConfig.RootCAs = roots

	got, err := exchangeEmbeddedSetupToken(
		context.Background(),
		client,
		server.URL,
		"test-oauth-cookie-value",
		"0123456789abcdef",
	)
	if err != nil {
		t.Fatalf("exchange token over HTTP/2: %v", err)
	}
	if got.MasterToken != masterToken {
		t.Fatalf("master token = %q, want %q", got.MasterToken, masterToken)
	}
}

func TestExchangeEmbeddedSetupToken(t *testing.T) {
	const (
		email       = "person@example.com"
		oauthToken  = "test-oauth-cookie-value"
		androidID   = "0123456789abcdef"
		masterToken = "oauth2rt_1/master=value=with=equals"
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.Header.Get("Accept-Encoding") != "identity" {
			t.Errorf("Accept-Encoding = %q, want identity", request.Header.Get("Accept-Encoding"))
		}
		if request.Header.Get("User-Agent") != "GoogleAuth/1.4" {
			t.Errorf("User-Agent = %q, want GoogleAuth/1.4", request.Header.Get("User-Agent"))
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		expected := map[string]string{
			"ACCESS_TOKEN":                 "1",
			"Email":                        googleAuthEmailHint,
			"Token":                        oauthToken,
			"androidId":                    androidID,
			"client_sig":                   googlePlayServicesSig,
			"callerSig":                    googlePlayServicesSig,
			"google_play_services_version": "240913000",
			"service":                      "ac2dm",
		}
		for key, want := range expected {
			if got := request.Form.Get(key); got != want {
				t.Errorf("form field %s = %q, want %q", key, got, want)
			}
		}

		_, _ = fmt.Fprintf(writer, "Token=%s\nEmail=%s\n", masterToken, email)
	}))
	defer server.Close()

	got, err := exchangeEmbeddedSetupToken(
		context.Background(),
		server.Client(),
		server.URL,
		oauthToken,
		androidID,
	)
	if err != nil {
		t.Fatalf("exchange token: %v", err)
	}
	if got.MasterToken != masterToken {
		t.Fatalf("master token was not parsed intact")
	}
	if got.Email != email {
		t.Fatalf("email = %q, want Google response email %q", got.Email, email)
	}
}

func TestExchangeEmbeddedSetupTokenRequiresGoogleResponseEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("Token=test-master-token\n"))
	}))
	defer server.Close()

	_, err := exchangeEmbeddedSetupToken(
		context.Background(),
		server.Client(),
		server.URL,
		"test-oauth-cookie-value",
		"0123456789abcdef",
	)
	if err == nil || !strings.Contains(err.Error(), "valid account email") {
		t.Fatalf("error = %v, want missing-email error", err)
	}
}

func TestExchangeEmbeddedSetupTokenRejectsBadAuthenticationWithoutLeakingCookie(t *testing.T) {
	const oauthToken = "test-secret-oauth-cookie"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("Error=BadAuthentication\n"))
	}))
	defer server.Close()

	_, err := exchangeEmbeddedSetupToken(
		context.Background(),
		server.Client(),
		server.URL,
		oauthToken,
		"0123456789abcdef",
	)
	if err == nil {
		t.Fatal("expected BadAuthentication error")
	}
	if strings.Contains(err.Error(), oauthToken) {
		t.Fatal("error contains the oauth cookie")
	}
	if !strings.Contains(err.Error(), "fresh cookie") {
		t.Fatalf("error = %q, want fresh-cookie guidance", err)
	}
}

func TestAddGoogleAccountUsesGoogleEmailAndReplacesCredential(t *testing.T) {
	restoreConfigGlobals(t)

	const (
		email        = "person@example.com"
		newAndroidID = "0000000000000000"
		masterToken  = "oauth2rt_1/generated-master-token"
	)
	oldCredential := url.Values{
		"Email":     {email},
		"Token":     {"old-token"},
		"androidId": {"fedcba9876543210"},
	}.Encode()
	AppConfig = DefaultConfig
	AppConfig.Account.Credentials = []string{oldCredential}
	AppConfig.Account.Selected = email
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, "Token=%s\nEmail=%s\n", masterToken, email)
	}))
	defer server.Close()

	validated := false
	dependencies := googleAccountAuthDependencies{
		client:       server.Client(),
		authEndpoint: server.URL,
		generateAndroidID: func() (string, error) {
			return newAndroidID, nil
		},
		validateCredential: func(credential string) error {
			values, err := url.ParseQuery(credential)
			if err != nil {
				return err
			}
			if values.Get("Token") != masterToken {
				return errors.New("generated credential has the wrong master token")
			}
			if values.Get("androidId") != newAndroidID {
				return errors.New("generated credential has the wrong Android ID")
			}
			if values.Get("Email") != email {
				return errors.New("generated credential did not use Google's email")
			}
			validated = true
			return nil
		},
	}

	manager := &ConfigManager{}
	gotEmail, err := manager.addGoogleAccount(
		context.Background(),
		"test-oauth-cookie-value",
		dependencies,
	)
	if err != nil {
		t.Fatalf("add Google account: %v", err)
	}
	if gotEmail != email {
		t.Fatalf("email = %q, want %q", gotEmail, email)
	}
	if !validated {
		t.Fatal("credential was not validated before persistence")
	}
	if len(AppConfig.Account.Credentials) != 1 {
		t.Fatalf("credentials count = %d, want 1", len(AppConfig.Account.Credentials))
	}
	values, err := url.ParseQuery(AppConfig.Account.Credentials[0])
	if err != nil {
		t.Fatalf("parse saved credential: %v", err)
	}
	if values.Get("Token") != masterToken {
		t.Fatal("saved credential does not contain the new master token")
	}
	if _, err := os.Stat(ConfigPath); err != nil {
		t.Fatalf("saved config: %v", err)
	}
}

func TestAddGoogleAccountDoesNotPersistFailedValidation(t *testing.T) {
	restoreConfigGlobals(t)

	AppConfig = DefaultConfig
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("Token=test-master-token\nEmail=person@example.com\n"))
	}))
	defer server.Close()

	dependencies := googleAccountAuthDependencies{
		client:       server.Client(),
		authEndpoint: server.URL,
		generateAndroidID: func() (string, error) {
			return "0123456789abcdef", nil
		},
		validateCredential: func(string) error {
			return errors.New("validation failed")
		},
	}

	manager := &ConfigManager{}
	_, err := manager.addGoogleAccount(
		context.Background(),
		"test-oauth-cookie-value",
		dependencies,
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(AppConfig.Account.Credentials) != 0 {
		t.Fatal("failed credential was added to memory")
	}
	if _, statErr := os.Stat(ConfigPath); !os.IsNotExist(statErr) {
		t.Fatalf("failed credential was written to disk: %v", statErr)
	}
}

func TestGenerateAndroidID(t *testing.T) {
	androidID, err := generateAndroidID()
	if err != nil {
		t.Fatalf("generate Android ID: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(androidID) {
		t.Fatalf("Android ID has unexpected format")
	}
}

func TestGetSettingsRedactsCredentials(t *testing.T) {
	restoreConfigGlobals(t)

	configMu.Lock()
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	AppConfig = Config{
		Account: AccountConfig{
			Credentials: []string{"Email=person%40example.com&Token=secret"},
			Selected:    "person@example.com",
		},
		Preferences: Preferences{Proxy: "http://proxy.example"},
	}
	configMu.Unlock()

	settings := (&ConfigManager{}).GetSettings()
	if settings.Proxy != "http://proxy.example" {
		t.Fatalf("proxy = %q, want configured proxy", settings.Proxy)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	serialized := string(encoded)
	if strings.Contains(serialized, "secret") {
		t.Fatalf("settings exposed credential data: %s", serialized)
	}
	if strings.Contains(serialized, "credentials") {
		t.Fatalf("settings included the credentials field: %s", serialized)
	}
}

func TestWriteConfigAtomicallyReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gotohp.config")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	if err := writeConfigAtomically(path, []byte("new")); err != nil {
		t.Fatalf("replace config: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced config: %v", err)
	}
	if string(contents) != "new" {
		t.Fatalf("config contents = %q, want new", contents)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat replaced config: %v", err)
		}
		if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("config permissions = %o, want 600", permissions)
		}
	}
}

func TestConcurrentSettingsWritesPersistLatestState(t *testing.T) {
	restoreConfigGlobals(t)

	configMu.Lock()
	AppConfig = DefaultConfig
	ConfigPath = filepath.Join(t.TempDir(), "gotohp.config")
	configMu.Unlock()

	manager := &ConfigManager{}
	var writes sync.WaitGroup
	for index := 1; index <= 16; index++ {
		index := index
		writes.Add(2)
		go func() {
			defer writes.Done()
			manager.SetProxy(fmt.Sprintf("proxy-%d", index))
		}()
		go func() {
			defer writes.Done()
			manager.SetUploadThreads(index)
		}()
	}
	writes.Wait()

	configMu.RLock()
	wantProxy := AppConfig.Preferences.Proxy
	wantUploadThreads := AppConfig.Preferences.UploadThreads
	path := ConfigPath
	configMu.RUnlock()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved := string(contents)
	if !strings.Contains(saved, fmt.Sprintf("proxy: %s", wantProxy)) {
		t.Fatalf("saved proxy does not match memory: want %q in %s", wantProxy, saved)
	}
	if !strings.Contains(saved, fmt.Sprintf("upload_threads: %d", wantUploadThreads)) {
		t.Fatalf("saved upload threads do not match memory: want %d in %s", wantUploadThreads, saved)
	}
}

func restoreConfigGlobals(t *testing.T) {
	t.Helper()
	configMu.Lock()
	previousConfig := AppConfig
	previousPath := ConfigPath
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		AppConfig = previousConfig
		ConfigPath = previousPath
		configMu.Unlock()
	})
}

func TestLooksLikeAuthString(t *testing.T) {
	cases := map[string]bool{
		"androidId=1&Email=a%40x.com&Token=t":            true,
		"  Email=a%40x.com&Token=t  ":                    true,
		"Email=a%40x.com":                                false,
		"oauth_token=abcdefghijklmnopqrstuvwxyz":         false,
		"4/0AbCdEfGhIjKlMnOpQrStUvWxYz-1234567890abcdef": false,
		"": false,
	}
	for in, want := range cases {
		if got := LooksLikeAuthString(in); got != want {
			t.Errorf("LooksLikeAuthString(%q) = %v, want %v", in, got, want)
		}
	}
}
