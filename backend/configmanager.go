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

// AccountConfig is the credential store shared by the GUI and the CLI.
type AccountConfig struct {
	Credentials []string `json:"credentials,omitempty" koanf:"credentials"`
	Selected    string   `json:"selected" koanf:"selected"`
}

// Preferences are GUI settings. The CLI never reads them; every CLI run is
// configured by flags alone.
type Preferences struct {
	Proxy                         string `json:"proxy" koanf:"proxy"`
	UseQuota                      bool   `json:"useQuota" koanf:"use_quota"`
	Saver                         bool   `json:"saver" koanf:"saver"`
	Recursive                     bool   `json:"recursive" koanf:"recursive"`
	ForceUpload                   bool   `json:"forceUpload" koanf:"force_upload"`
	PairLivePhotos                bool   `json:"pairLivePhotos" koanf:"pair_live_photos"`
	SkipIncompleteLivePhotos      bool   `json:"skipIncompleteLivePhotos" koanf:"skip_incomplete_live_photos"`
	UpdateExistingPhotosToLive    bool   `json:"updateExistingPhotosToLive" koanf:"update_existing_photos_to_live"`
	UploadThreads                 int    `json:"uploadThreads" koanf:"upload_threads"`
	DeleteFromHost                bool   `json:"deleteFromHost" koanf:"delete_from_host"`
	DisableUnsupportedFilesFilter bool   `json:"disableUnsupportedFilesFilter" koanf:"disable_unsupported_files_filter"`
	SetDateFromFilename           bool   `json:"setDateFromFilename" koanf:"set_date_from_filename"`
	ExcludePattern                string `json:"excludePattern" koanf:"exclude_pattern"`
	// AlbumName and AlbumAutoMode are per-session choices and are never persisted.
	AlbumName     string `json:"albumName" koanf:"-"`
	AlbumAutoMode bool   `json:"albumAutoMode" koanf:"-"`
}

// Config is the on-disk layout of gotohp.config.
type Config struct {
	Account     AccountConfig `json:"account" koanf:"account"`
	Preferences Preferences   `json:"preferences" koanf:"preferences"`
}

// legacyConfig is the pre-sectioned flat layout, kept only for migration.
type legacyConfig struct {
	Credentials                   []string `koanf:"credentials"`
	Selected                      string   `koanf:"selected"`
	Proxy                         string   `koanf:"proxy"`
	UseQuota                      bool     `koanf:"use_quota"`
	Saver                         bool     `koanf:"saver"`
	Recursive                     bool     `koanf:"recursive"`
	ForceUpload                   bool     `koanf:"force_upload"`
	PairLivePhotos                bool     `koanf:"pair_live_photos"`
	SkipIncompleteLivePhotos      bool     `koanf:"skip_incomplete_live_photos"`
	UpdateExistingPhotosToLive    bool     `koanf:"update_existing_photos_to_live"`
	UploadThreads                 int      `koanf:"upload_threads"`
	DeleteFromHost                bool     `koanf:"delete_from_host"`
	DisableUnsupportedFilesFilter bool     `koanf:"disable_unsupported_files_filter"`
	SetDateFromFilename           bool     `koanf:"set_date_from_filename"`
	ExcludePattern                string   `koanf:"exclude_pattern"`
}

func (l legacyConfig) toConfig() Config {
	return Config{
		Account: AccountConfig{Credentials: l.Credentials, Selected: l.Selected},
		Preferences: Preferences{
			Proxy:                         l.Proxy,
			UseQuota:                      l.UseQuota,
			Saver:                         l.Saver,
			Recursive:                     l.Recursive,
			ForceUpload:                   l.ForceUpload,
			PairLivePhotos:                l.PairLivePhotos,
			SkipIncompleteLivePhotos:      l.SkipIncompleteLivePhotos,
			UpdateExistingPhotosToLive:    l.UpdateExistingPhotosToLive,
			UploadThreads:                 l.UploadThreads,
			DeleteFromHost:                l.DeleteFromHost,
			DisableUnsupportedFilesFilter: l.DisableUnsupportedFilesFilter,
			SetDateFromFilename:           l.SetDateFromFilename,
			ExcludePattern:                l.ExcludePattern,
		},
	}
}

type AccountSummary struct {
	Email             string `json:"email"`
	NeedsTokenBinding bool   `json:"needsTokenBinding"`
}

type AccountsState struct {
	Accounts []AccountSummary `json:"accounts"`
	Selected string           `json:"selected"`
}

type ConfigManager struct{}

var (
	configMu           sync.RWMutex
	AppConfig          Config
	ConfigPath         string
	DefaultPreferences = Preferences{
		SkipIncompleteLivePhotos: true,
		UploadThreads:            3,
	}
	DefaultConfig = Config{Preferences: DefaultPreferences}
)

// ParseAuthString parses an auth string and returns url.Values (exported for CLI use)
func ParseAuthString(authString string) (url.Values, error) {
	return url.ParseQuery(authString)
}

func (g *ConfigManager) SetProxy(proxy string) {
	updateAppConfig(func(config *Config) {
		config.Preferences.Proxy = proxy
	})
}

func (g *ConfigManager) SetSelected(email string) {
	updateAppConfig(func(config *Config) {
		config.Account.Selected = email
	})
}

func (g *ConfigManager) SetUseQuota(useQuota bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.UseQuota = useQuota
	})
}

func (g *ConfigManager) SetSaver(saver bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.Saver = saver
	})
}

func (g *ConfigManager) SetRecursive(recursive bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.Recursive = recursive
	})
}

func (g *ConfigManager) SetForceUpload(forceUpload bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.ForceUpload = forceUpload
	})
}

func (g *ConfigManager) SetPairLivePhotos(pairLivePhotos bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.PairLivePhotos = pairLivePhotos
	})
}

func (g *ConfigManager) SetSkipIncompleteLivePhotos(skipIncompleteLivePhotos bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.SkipIncompleteLivePhotos = skipIncompleteLivePhotos
	})
}

func (g *ConfigManager) SetUpdateExistingPhotosToLive(updateExistingPhotosToLive bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.UpdateExistingPhotosToLive = updateExistingPhotosToLive
	})
}

func (g *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.DeleteFromHost = deleteFromHost
	})
}

func (g *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	})
}

func (g *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		return
	}
	updateAppConfig(func(config *Config) {
		config.Preferences.UploadThreads = uploadThreads
	})
}

func (g *ConfigManager) SetAlbumName(albumName string) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.Preferences.AlbumName = strings.TrimSpace(albumName)
}

func (g *ConfigManager) GetAlbumName() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.Preferences.AlbumName
}

func (g *ConfigManager) SetAlbumAutoMode(autoMode bool) {
	configMu.Lock()
	defer configMu.Unlock()
	AppConfig.Preferences.AlbumAutoMode = autoMode
	// Don't persist to disk - this is per-session like AlbumName
}

func (g *ConfigManager) GetAlbumAutoMode() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.Preferences.AlbumAutoMode
}

func (g *ConfigManager) SetSetDateFromFilename(v bool) {
	updateAppConfig(func(config *Config) {
		config.Preferences.SetDateFromFilename = v
	})
}

func (g *ConfigManager) SetExcludePattern(pattern string) {
	updateAppConfig(func(config *Config) {
		config.Preferences.ExcludePattern = pattern
	})
}

func (g *ConfigManager) GetExcludePattern() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.Preferences.ExcludePattern
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

	configMu.Lock()
	defer configMu.Unlock()

	// Check for duplicate email in existing credentials
	for _, cred := range AppConfig.Account.Credentials {
		existingParams, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}
		if existingParams.Get("Email") == email {
			return fmt.Errorf("auth string with email %s already exists", email)
		}
	}

	// If validation passed, add the new credentials
	AppConfig.Account.Credentials = append(AppConfig.Account.Credentials, newAuthString)
	AppConfig.Account.Selected = email
	_ = saveAppConfigLocked()
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

	configMu.Lock()
	defer configMu.Unlock()

	for i, cred := range AppConfig.Account.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue
		}
		if params.Get("Email") != email {
			continue
		}
		params.Set("token_binding_alias", alias)
		AppConfig.Account.Credentials[i] = params.Encode()
		return saveAppConfigLocked()
	}

	return fmt.Errorf("no credentials found for email %s", email)
}

func (g *ConfigManager) RemoveCredentials(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	configMu.Lock()
	defer configMu.Unlock()

	// Find and remove the credential with matching email
	found := false
	var updatedCredentials []string

	for _, cred := range AppConfig.Account.Credentials {
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
	AppConfig.Account.Credentials = updatedCredentials

	// If we're removing the currently selected credential, clear the selection
	if AppConfig.Account.Selected == email {
		AppConfig.Account.Selected = ""
	}

	_ = saveAppConfigLocked()
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

// determineConfigPath picks the config location when none was given explicitly:
// a portable gotohp.config next to the executable wins over the user config dir.
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

//wails:ignore
func (g *ConfigManager) GetConfig() Config {
	ensureConfigLoaded()
	return currentConfig()
}

// currentConfig returns a snapshot of the loaded config.
func currentConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig
}

func (g *ConfigManager) GetSettings() Preferences {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.Preferences
}

// SessionUploadOptions builds upload options from the GUI's current preferences.
//
//wails:ignore
func (g *ConfigManager) SessionUploadOptions() UploadOptions {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig.Preferences.UploadOptions()
}

func (g *ConfigManager) GetAccounts() AccountsState {
	ensureConfigLoaded()
	configMu.RLock()
	defer configMu.RUnlock()

	state := AccountsState{
		Accounts: make([]AccountSummary, 0, len(AppConfig.Account.Credentials)),
		Selected: AppConfig.Account.Selected,
	}
	for _, credential := range AppConfig.Account.Credentials {
		values, err := url.ParseQuery(credential)
		if err != nil || values.Get("Email") == "" {
			continue
		}
		state.Accounts = append(state.Accounts, AccountSummary{
			Email:             values.Get("Email"),
			NeedsTokenBinding: credentialNeedsTokenBinding(values),
		})
	}
	return state
}

func ensureConfigLoaded() {
	configMu.RLock()
	loaded := ConfigPath != ""
	configMu.RUnlock()
	if loaded {
		return
	}

	configMu.Lock()
	defer configMu.Unlock()
	if ConfigPath == "" {
		_ = loadConfigLocked()
	}
}

// LoadConfig loads settings from path, or from the default location when path
// is empty. It replaces any previously loaded config.
func LoadConfig(path string) error {
	configMu.Lock()
	defer configMu.Unlock()
	ConfigPath = path
	return loadConfigLocked()
}

func loadConfigLocked() error {
	if ConfigPath == "" {
		determineConfigPath()
	}

	file, _ := os.ReadFile(ConfigPath)
	if len(file) == 0 {
		AppConfig = DefaultConfig
	} else {
		AppConfig = loadAppConfig()
		_ = os.Chmod(ConfigPath, 0o600)
	}

	return nil
}

func updateAppConfig(update func(*Config)) {
	configMu.Lock()
	defer configMu.Unlock()

	update(&AppConfig)
	_ = saveAppConfigLocked()
}

// saveAppConfigLocked persists AppConfig while the caller holds configMu for writing.
func saveAppConfigLocked() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(AppConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0o700); err != nil {
		return err
	}
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		fmt.Println(err)
		return err
	}

	return writeConfigAtomically(ConfigPath, b)
}

func writeConfigAtomically(path string, contents []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".gotohp-config-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func loadAppConfig() Config {
	c := DefaultConfig
	k := koanf.New(".")
	if err := k.Load(file.Provider(ConfigPath), yaml.Parser()); err != nil {
		log.Printf("error parsing app config: %v", err)
		return DefaultConfig
	}
	if isLegacyConfig(k) {
		return migrateLegacyConfig(k)
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		log.Printf("error unmarshaling app config: %v", err)
		return DefaultConfig
	}

	if !k.Exists("preferences.skip_incomplete_live_photos") {
		c.Preferences.SkipIncompleteLivePhotos = DefaultPreferences.SkipIncompleteLivePhotos
	}

	if c.Preferences.UploadThreads < 1 {
		c.Preferences.UploadThreads = DefaultPreferences.UploadThreads
	}

	return c
}

// isLegacyConfig reports whether the loaded file uses the flat pre-sectioned layout.
func isLegacyConfig(k *koanf.Koanf) bool {
	if k.Exists("account") || k.Exists("preferences") {
		return false
	}
	for _, key := range []string{"credentials", "selected", "upload_threads", "proxy", "recursive"} {
		if k.Exists(key) {
			return true
		}
	}
	return false
}

// migrateLegacyConfig converts a flat config into the sectioned layout and
// rewrites the file so the migration happens once.
func migrateLegacyConfig(k *koanf.Koanf) Config {
	legacy := legacyConfig{
		SkipIncompleteLivePhotos: DefaultPreferences.SkipIncompleteLivePhotos,
		UploadThreads:            DefaultPreferences.UploadThreads,
	}
	if err := k.Unmarshal("", &legacy); err != nil {
		log.Printf("error migrating legacy app config: %v", err)
		return DefaultConfig
	}
	c := legacy.toConfig()
	if c.Preferences.UploadThreads < 1 {
		c.Preferences.UploadThreads = DefaultPreferences.UploadThreads
	}
	previous := AppConfig
	AppConfig = c
	if err := saveAppConfigLocked(); err != nil {
		log.Printf("error saving migrated app config: %v", err)
	}
	AppConfig = previous
	return c
}
