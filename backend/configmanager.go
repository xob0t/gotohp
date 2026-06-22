package backend

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	_ "modernc.org/sqlite"
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
	AlbumName                     string   `json:"albumName" koanf:"album_name"`
	AlbumAutoMode                 bool     `json:"albumAutoMode" koanf:"album_auto_mode"`
	SetDateFromFilename           bool     `json:"setDateFromFilename" koanf:"set_date_from_filename"`
	ExcludePattern                string   `json:"excludePattern" koanf:"exclude_pattern"`
}

type ConfigManager struct{}

var (
	configMu      sync.RWMutex
	AppConfig     Config
	UploadRunning bool = false
	ConfigPath    string
	DefaultConfig = Config{
		UploadThreads: 3,
	}
)

// ParseAuthString parses an auth string and returns url.Values (exported for CLI use)
func ParseAuthString(authString string) (url.Values, error) {
	return url.ParseQuery(authString)
}

func (g *ConfigManager) SetProxy(proxy string) {
	AppConfig.Proxy = proxy
	_ = saveAppConfig()
}

func (g *ConfigManager) SetSelected(email string) {
	// Parse the auth string
	AppConfig.Selected = email
	_ = saveAppConfig()
}

func (g *ConfigManager) SetUseQuota(useQuota bool) {
	AppConfig.UseQuota = useQuota
	_ = saveAppConfig()
}

func (g *ConfigManager) SetSaver(saver bool) {
	AppConfig.Saver = saver
	_ = saveAppConfig()
}

func (g *ConfigManager) SetRecursive(recursive bool) {
	AppConfig.Recursive = recursive
	_ = saveAppConfig()
}

func (g *ConfigManager) SetForceUpload(forceUpload bool) {
	AppConfig.ForceUpload = forceUpload
	_ = saveAppConfig()
}

func (g *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	AppConfig.DeleteFromHost = deleteFromHost
	_ = saveAppConfig()
}

func (g *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	AppConfig.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	_ = saveAppConfig()
}

func (g *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		return
	}
	AppConfig.UploadThreads = uploadThreads
	_ = saveAppConfig()
}

func (g *ConfigManager) SetAlbumName(albumName string) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.AlbumName = strings.TrimSpace(albumName)
}

func (g *ConfigManager) GetAlbumName() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumName
}

func (g *ConfigManager) SetAlbumAutoMode(autoMode bool) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.AlbumAutoMode = autoMode
	// Don't persist to disk - this is per-session like AlbumName
}

func (g *ConfigManager) GetAlbumAutoMode() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumAutoMode
}

// GetAlbumConfig returns album name and auto mode atomically
func GetAlbumConfig() (albumName string, autoMode bool) {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.AlbumName, AppConfig.AlbumAutoMode
}

func (g *ConfigManager) SetSetDateFromFilename(v bool) {
	AppConfig.SetDateFromFilename = v
	_ = saveAppConfig()
}

func (g *ConfigManager) SetExcludePattern(pattern string) {
	AppConfig.ExcludePattern = pattern
	_ = saveAppConfig()
}

func (g *ConfigManager) GetExcludePattern() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.ExcludePattern
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
	_ = saveAppConfig()
	return nil
}

func (g *ConfigManager) CredentialNeedsTokenBinding(authString string) bool {
	params, err := url.ParseQuery(authString)
	if err != nil {
		return false
	}
	return credentialNeedsTokenBinding(params)
}

func (g *ConfigManager) AddTokenBindingAliasFromADB(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	alias, err := extractTokenBindingAliasFromADB(email)
	if err != nil {
		return err
	}

	for i, cred := range AppConfig.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue
		}
		if params.Get("Email") != email {
			continue
		}
		params.Set("token_binding_alias", alias)
		AppConfig.Credentials[i] = params.Encode()
		return saveAppConfig()
	}

	return fmt.Errorf("no credentials found for email %s", email)
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

	_ = saveAppConfig()
	return nil
}

func credentialNeedsTokenBinding(params url.Values) bool {
	if params.Get("token_binding_alias") != "" {
		return false
	}
	return params.Get("assertion_jwt") != "" ||
		params.Get("check_tb_upgrade_eligible") != ""
}

// accountsCEDBPath is the credential-encrypted AccountManager database for the
// primary (user 0) Android profile.
const accountsCEDBPath = "/data/system_ce/0/accounts_ce.db"

// errADBRootUnavailable signals that the device could not be read because root
// access was denied, as opposed to the database simply not containing the key.
var errADBRootUnavailable = errors.New("root access unavailable")

func extractTokenBindingAliasFromADB(email string) (string, error) {
	if _, err := exec.LookPath("adb"); err != nil {
		return "", fmt.Errorf("adb was not found in PATH")
	}

	escapedEmail := strings.ReplaceAll(email, "'", "''")
	query := fmt.Sprintf(
		"select extras.value from extras join accounts on accounts._id=extras.accounts_id where accounts.name='%s' and extras.key='lstBindingKeyAlias';",
		escapedEmail,
	)

	devices, err := listADBDevices()
	if err != nil {
		return "", err
	}

	var failures []string
	var reachableRoot bool
	for _, device := range devices {
		_ = exec.Command("adb", "-s", device, "root").Run()

		alias, rooted, err := readTokenBindingAliasFromDevice(device, query)
		if rooted {
			reachableRoot = true
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", device, cleanADBError(err.Error())))
			continue
		}

		alias = strings.TrimSpace(alias)
		if alias == "" {
			failures = append(failures, fmt.Sprintf("%s: no token binding key for %s", device, email))
			continue
		}
		if !strings.HasPrefix(alias, tokenBindingECDSAAliasPrefix) {
			failures = append(failures, fmt.Sprintf("%s: unsupported token binding key format", device))
			continue
		}

		return alias, nil
	}

	if reachableRoot {
		return "", fmt.Errorf("token binding alias not found for %s on any connected adb device (%s)", email, strings.Join(failures, "; "))
	}
	return "", fmt.Errorf("could not read Android AccountManager on any connected adb device; root is required (%s)", strings.Join(failures, "; "))
}

// readTokenBindingAliasFromDevice pulls the AccountManager database from a single
// device and runs the lookup against the local copy. The pulled database is
// sensitive (it holds auth material for every account on the device), so the
// temporary copy is always removed via defer — including on a panic. The rooted
// return reports whether the device was reachable with root, used to produce an
// accurate error message.
func readTokenBindingAliasFromDevice(device, query string) (alias string, rooted bool, err error) {
	dbPath, cleanup, err := pullAccountsDB(device)
	if err != nil {
		// A non-root failure means we did reach the device with root but
		// something else went wrong (e.g. the db file is missing).
		return "", !errors.Is(err, errADBRootUnavailable), err
	}
	defer cleanup()

	alias, err = queryTokenBindingAlias(dbPath, query)
	if err != nil {
		return "", true, err
	}
	return alias, true, nil
}

func listADBDevices() ([]string, error) {
	out, err := exec.Command("adb", "devices").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list adb devices: %s", cleanADBError(string(out)))
	}

	var devices []string
	var unavailable []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "List" {
			continue
		}
		switch fields[1] {
		case "device":
			devices = append(devices, fields[0])
		case "offline", "unauthorized", "no permissions":
			unavailable = append(unavailable, fmt.Sprintf("%s is %s", fields[0], fields[1]))
		default:
			unavailable = append(unavailable, fmt.Sprintf("%s is %s", fields[0], fields[1]))
		}
	}

	if len(devices) == 0 {
		if len(unavailable) > 0 {
			return nil, fmt.Errorf("no usable adb devices found (%s)", strings.Join(unavailable, "; "))
		}
		return nil, fmt.Errorf("no adb devices found")
	}

	return devices, nil
}

// pullAccountsDB streams the AccountManager database off the device via root and
// writes it to a local temporary directory. Modern Android no longer ships the
// sqlite3 binary, so the database is queried locally instead of on-device. The
// returned cleanup function removes the temporary files.
func pullAccountsDB(device string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "gotohp-adb-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	mainDest := filepath.Join(tmpDir, "accounts_ce.db")
	if err := streamDeviceFile(device, accountsCEDBPath, mainDest); err != nil {
		cleanup()
		return "", nil, err
	}

	// accounts_ce.db is typically in WAL mode; copy the companion files so the
	// local query observes the latest writes. They may not exist, so ignore
	// failures here.
	_ = streamDeviceFile(device, accountsCEDBPath+"-wal", mainDest+"-wal")
	_ = streamDeviceFile(device, accountsCEDBPath+"-shm", mainDest+"-shm")

	return mainDest, cleanup, nil
}

// streamDeviceFile copies a single root-owned file off the device to localPath.
// adb exec-out is used (rather than adb shell) to avoid the CRLF translation
// that would corrupt the binary database.
func streamDeviceFile(device, remotePath, localPath string) error {
	cmd := exec.Command("adb", "-s", device, "exec-out", "su", "-c", fmt.Sprintf("cat %q", remotePath))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	// Some su builds exit 0 even when cat fails, so also treat stderr-with-no-
	// output as a failure.
	if runErr != nil || (stderr.Len() > 0 && stdout.Len() == 0) {
		msg := cleanADBError(stderr.String())
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if isADBRootFailure(msg) {
			return fmt.Errorf("%w: %s", errADBRootUnavailable, msg)
		}
		return fmt.Errorf("failed to read %s: %s", remotePath, msg)
	}

	if err := os.WriteFile(localPath, stdout.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write local copy: %w", err)
	}
	return nil
}

// isADBRootFailure reports whether a device error indicates missing/denied root
// rather than a missing file (which means root worked but the data is absent).
func isADBRootFailure(msg string) bool {
	m := strings.ToLower(msg)
	if strings.Contains(m, "no such file") {
		return false
	}
	return strings.Contains(m, "su:") ||
		strings.Contains(m, "permission denied") ||
		strings.Contains(m, "not allowed") ||
		strings.Contains(m, "inaccessible or not found")
}

// queryTokenBindingAlias opens the pulled database locally with a pure-Go SQLite
// driver and runs the lookup query. The local copy is private to this process,
// so it is opened read-write to allow WAL recovery.
func queryTokenBindingAlias(dbPath, query string) (string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open accounts db: %w", err)
	}
	defer func() { _ = db.Close() }()

	var alias string
	if err := db.QueryRow(query).Scan(&alias); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to query token binding alias: %w", err)
	}
	return alias, nil
}

func cleanADBError(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return "adb command failed"
	}
	return strings.Join(strings.Fields(out), " ")
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
	// Don't reload if already loaded
	if len(AppConfig.Credentials) == 0 && AppConfig.UploadThreads == 0 {
		_ = LoadConfig()
	}
	return AppConfig
}

// LoadConfig loads the configuration (exported for CLI use)
func LoadConfig() error {
	determineConfigPath()

	file, _ := os.ReadFile(ConfigPath)
	if len(file) == 0 {
		AppConfig = DefaultConfig
	} else {
		AppConfig = loadAppConfig()
	}

	return nil
}

func saveAppConfig() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(AppConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0o755); err != nil {
		return err
	}
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = os.WriteFile(ConfigPath, b, 0o644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func loadAppConfig() Config {
	var c Config
	k := koanf.New(".")
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
