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

type GlobalSettings struct {
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

type ConfigService struct{}

var GlobalSettingsConfig GlobalSettings
var UploadRunning bool = false
var ConfigDir string = filepath.Join(GetUserDir(), "/.config/gotohp")
var ConfigPath string = filepath.Join(ConfigDir, "config.yaml")
var DefaultConfig = GlobalSettings{
	UploadThreads: 3,
}

func (g *ConfigService) SetProxy(proxy string) {
	GlobalSettingsConfig.Proxy = proxy
	SaveGlobalConfig()
}

func (g *ConfigService) SetSelected(email string) {
	// Parse the auth string
	GlobalSettingsConfig.Selected = email
	SaveGlobalConfig()
}

func (g *ConfigService) SetUseQuota(useQuota bool) {
	GlobalSettingsConfig.UseQuota = useQuota
	SaveGlobalConfig()
}

func (g *ConfigService) SetSaver(saver bool) {
	GlobalSettingsConfig.Saver = saver
	SaveGlobalConfig()
}

func (g *ConfigService) SetRecursive(recursive bool) {
	GlobalSettingsConfig.Recursive = recursive
	SaveGlobalConfig()
}

func (g *ConfigService) SetForceUpload(forceUpload bool) {
	GlobalSettingsConfig.ForceUpload = forceUpload
	SaveGlobalConfig()
}

func (g *ConfigService) SetDeleteFromHost(deleteFromHost bool) {
	GlobalSettingsConfig.DeleteFromHost = deleteFromHost
	SaveGlobalConfig()
}

func (g *ConfigService) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	GlobalSettingsConfig.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	SaveGlobalConfig()
}

func (g *ConfigService) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		GlobalSettingsConfig.UploadThreads = uploadThreads
		SaveGlobalConfig()
	}
}

func (g *ConfigService) AddCredentials(newAuthString string) error {
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
	for _, cred := range GlobalSettingsConfig.Credentials {
		existingParams, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}
		if existingParams.Get("Email") == email {
			return fmt.Errorf("auth string with email %s already exists", email)
		}
	}

	// If validation passed, add the new credentials
	GlobalSettingsConfig.Credentials = append(GlobalSettingsConfig.Credentials, newAuthString)
	GlobalSettingsConfig.Selected = email
	SaveGlobalConfig()
	return nil
}

func (g *ConfigService) RemoveCredentials(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Find and remove the credential with matching email
	found := false
	var updatedCredentials []string

	for _, cred := range GlobalSettingsConfig.Credentials {
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
	GlobalSettingsConfig.Credentials = updatedCredentials

	// If we're removing the currently selected credential, clear the selection
	if GlobalSettingsConfig.Selected == email {
		GlobalSettingsConfig.Selected = ""
	}

	SaveGlobalConfig()
	return nil
}

func (g *ConfigService) GetConfig() GlobalSettings {
	configExists := Exists(ConfigPath)
	if !configExists {
		fmt.Println("Created a new user settings config")
		GlobalSettingsConfig = DefaultConfig
	}
	file, _ := os.ReadFile(ConfigPath)
	if len(file) == 0 {
		fmt.Println("config file is empty")
		GlobalSettingsConfig = DefaultConfig
	} else {
		GlobalSettingsConfig, _ = parseGlobalConfig()
	}

	log.Println("Config", GlobalSettingsConfig)
	return GlobalSettingsConfig
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

func SaveGlobalConfig() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(GlobalSettingsConfig, "koanf"), nil)
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

func parseGlobalConfig() (GlobalSettings, error) {
	var c GlobalSettings
	var k = koanf.New(".")
	if err := k.Load(file.Provider(ConfigPath), yaml.Parser()); err != nil {
		log.Printf("error loading global config: %v", err)
		return DefaultConfig, err
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		log.Printf("error Unmarshaling global config: %v", err)
		return DefaultConfig, err
	}

	if c.UploadThreads < 1 {
		c.UploadThreads = DefaultConfig.UploadThreads
	}

	return c, nil
}
