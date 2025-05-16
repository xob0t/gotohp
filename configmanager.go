package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Credentials                   []string `json:"credentials" koanf:"credentials"`
	Selected                      string   `json:"selected" koanf:"selected"`
	Proxy                         string   `json:"proxy" koanf:"proxy"`
	UseQuota                      bool     `json:"useQuota" koanf:"use_quota"`
	Saver                         bool     `json:"saver" koanf:"saver"`
	Recursive                     bool     `json:"recursive" koanf:"recursive"`
	ForceUpload                   bool     `json:"forceUpload" koanf:"force_upload"`
	UploadThreads                 int      `json:"uploadThreads" koanf:"upload_threads"`
	DeleteFromHost                bool     `json:"deleteFromHost" koanf:"delete_from_host"`
	DisableUnsupportedFilesFilter bool     `json:"disableUnsupportedFilesFilter" koanf:"disable_unsupported_files_filter"`
}

type ConfigManager struct{}

var AppConfig Config
var UploadRunning bool = false
var ConfigDir string = filepath.Join(GetUserDir(), "/.config/gotohp")
var ConfigPath string = filepath.Join(ConfigDir, "config.yaml")
var DefaultAppConfig = Config{
	UploadThreads: 3,
}

func (m *ConfigManager) SetProxy(proxy string) {
	AppConfig.Proxy = proxy
	AppConfig.Save()
}

func (m *ConfigManager) SetSelected(email string) {
	// Parse the auth string
	AppConfig.Selected = email
	AppConfig.Save()
}

func (m *ConfigManager) SetUseQuota(useQuota bool) {
	AppConfig.UseQuota = useQuota
	AppConfig.Save()
}

func (m *ConfigManager) SetSaver(saver bool) {
	AppConfig.Saver = saver
	AppConfig.Save()
}

func (m *ConfigManager) SetRecursive(recursive bool) {
	AppConfig.Recursive = recursive
	AppConfig.Save()
}

func (m *ConfigManager) SetForceUpload(forceUpload bool) {
	AppConfig.ForceUpload = forceUpload
	AppConfig.Save()
}

func (m *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	AppConfig.DeleteFromHost = deleteFromHost
	AppConfig.Save()
}

func (m *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	AppConfig.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	AppConfig.Save()
}

func (m *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads >= 1 {
		AppConfig.UploadThreads = uploadThreads
		AppConfig.Save()
	}
}

func (m *ConfigManager) AddCredentials(newAuthString string) error {
	// Required fields that must be present in the auth string
	requiredFields := []string{
		"androidId",
		"app",
		"client_sig",
		"Email",
		"Token",
		"lang",
		"service",
	}

	// Parse the auth string
	params, err := url.ParseQuery(newAuthString)
	if err != nil {
		return fmt.Errorf("invalid auth string format: %v", err)
	}

	// Validate required fields
	var missingFields []string
	for _, field := range requiredFields {
		if params.Get(field) == "" {
			missingFields = append(missingFields, field)
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("auth string missing required fields: %v", missingFields)
	}

	// Get and validate email
	email := params.Get("Email")
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Check for duplicate email in existing credentials
	for _, cred := range AppConfig.Credentials {
		existingParams, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}
		if existingParams.Get("Email") == email {
			return fmt.Errorf("auth string with email %s already exists", email)
		}
	}

	// If validation passed, add the new credentials
	AppConfig.Credentials = append(AppConfig.Credentials, newAuthString)
	AppConfig.Selected = email
	AppConfig.Save()
	return nil
}

func (m *ConfigManager) RemoveCredentials(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Find and remove the credential with matching email
	found := false
	var updatedCredentials []string

	for _, cred := range AppConfig.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}

		if params.Get("Email") == email {
			found = true
			continue // skip this credential (effectively removing it)
		}

		updatedCredentials = append(updatedCredentials, cred)
	}

	if !found {
		return fmt.Errorf("no credentials found for email %s", email)
	}

	// Update the configuration
	AppConfig.Credentials = updatedCredentials

	// If we're removing the currently selected credential, clear the selection
	if AppConfig.Selected == email {
		AppConfig.Selected = ""
	}

	AppConfig.Save()
	return nil
}

func (m *ConfigManager) GetConfig() Config {
	if !Exists(ConfigPath) {
		fmt.Println("Created a new user settings config")
		AppConfig = DefaultAppConfig
		return AppConfig
	}

	file, err := os.ReadFile(ConfigPath)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		AppConfig = DefaultAppConfig
		return AppConfig
	}

	if len(file) == 0 {
		fmt.Println("Config file is empty")
		AppConfig = DefaultAppConfig
		return AppConfig
	}

	AppConfig, err := AppConfig.Load()
	if err != nil {
		fmt.Println("Error loading config:", err)
		AppConfig = DefaultAppConfig
		return AppConfig
	}

	log.Println("Config", AppConfig)
	return AppConfig
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func GetUserDir() string {
	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return dirname
}

func (m *Config) Save() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(AppConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	os.MkdirAll(ConfigDir, 0666)
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = os.WriteFile(ConfigPath, b, os.ModePerm)

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (m *Config) Load() (Config, error) {
	var c Config
	k := koanf.New(".")

	if err := k.Load(file.Provider(ConfigPath), yaml.Parser()); err != nil {
		return c, fmt.Errorf("error loading config file: %w", err)
	}

	if err := k.Unmarshal("", &c); err != nil {
		return c, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if c.UploadThreads < 1 {
		c.UploadThreads = DefaultAppConfig.UploadThreads
	}

	return c, nil
}
