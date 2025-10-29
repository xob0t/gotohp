package backend

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
var ConfigPath string
var DefaultConfig = Config{
	UploadThreads: 3,
}

func (g *ConfigManager) SetProxy(proxy string) {
	AppConfig.Proxy = proxy
	saveAppConfig()
}

func (g *ConfigManager) SetSelected(email string) {
	// Parse the auth string
	AppConfig.Selected = email
	saveAppConfig()
}

func (g *ConfigManager) SetUseQuota(useQuota bool) {
	AppConfig.UseQuota = useQuota
	saveAppConfig()
}

func (g *ConfigManager) SetSaver(saver bool) {
	AppConfig.Saver = saver
	saveAppConfig()
}

func (g *ConfigManager) SetRecursive(recursive bool) {
	AppConfig.Recursive = recursive
	saveAppConfig()
}

func (g *ConfigManager) SetForceUpload(forceUpload bool) {
	AppConfig.ForceUpload = forceUpload
	saveAppConfig()
}

func (g *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	AppConfig.DeleteFromHost = deleteFromHost
	saveAppConfig()
}

func (g *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	AppConfig.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	saveAppConfig()
}

func (g *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		return
	}
	AppConfig.UploadThreads = uploadThreads
	saveAppConfig()
}

func (g *ConfigManager) AddCredentials(newAuthString string) error {
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
	saveAppConfig()
	return nil
}

func (g *ConfigManager) RemoveCredentials(email string) error {
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

	saveAppConfig()
	return nil
}

func determineConfigPath() {
	// First try portable config in executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		portableConfigPath := filepath.Join(exeDir, "gotohp.config")

		// If config exists in executable directory, use it
		if _, err := os.Stat(portableConfigPath); err == nil {
			ConfigPath = portableConfigPath
			return
		}
	}

	// Fall back to default location
	userConfigDir := filepath.Join(getUserConfigDir(), "/gotohp")
	ConfigPath = filepath.Join(userConfigDir, "gotohp.config")
}

func getUserConfigDir() string {
	dirname, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	return dirname
}

func (g *ConfigManager) GetConfig() Config {
	determineConfigPath()

	file, _ := os.ReadFile(ConfigPath)
	if len(file) == 0 {
		fmt.Println("config file is empty")
		AppConfig = DefaultConfig
	} else {
		AppConfig = loadAppConfig()
	}

	log.Println("Config", AppConfig)
	return AppConfig
}

func saveAppConfig() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(AppConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	os.MkdirAll(filepath.Dir(ConfigPath), 0755)
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

func loadAppConfig() Config {
	var c Config
	var k = koanf.New(".")
	if err := k.Load(file.Provider(ConfigPath), yaml.Parser()); err != nil {
		log.Printf("error parsing app config: %v", err)
		return DefaultConfig
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		log.Printf("error unmarshaling app config: %v", err)
		return DefaultConfig
	}

	if c.UploadThreads < 1 {
		c.UploadThreads = DefaultConfig.UploadThreads
	}

	return c
}
