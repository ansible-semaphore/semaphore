package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/github"

	"github.com/gorilla/securecookie"
)

// Cookie is a runtime generated secure cookie used for authentication
var Cookie *securecookie.SecureCookie

// WebHostURL is the public route to the semaphore server
var WebHostURL *url.URL

type DbDriver string

const (
	DbDriverMySQL    DbDriver = "mysql"
	DbDriverBolt     DbDriver = "bolt"
	DbDriverPostgres DbDriver = "postgres"
)

type DbConfig struct {
	Dialect DbDriver `json:"-"`

	Hostname string            `json:"host"`
	Username string            `json:"user"`
	Password string            `json:"pass"`
	DbName   string            `json:"name"`
	Options  map[string]string `json:"options"`
}

type ldapMappings struct {
	DN   string `json:"dn"`
	Mail string `json:"mail"`
	UID  string `json:"uid"`
	CN   string `json:"cn"`
}

// ConfigType mapping between Config and the json file that sets it
type ConfigType struct {
	MySQL    DbConfig `json:"mysql"`
	BoltDb   DbConfig `json:"bolt"`
	Postgres DbConfig `json:"postgres"`

	Dialect DbDriver `json:"dialect"`

	// Format `:port_num` eg, :3000
	// if : is missing it will be corrected
	Port string `json:"port"`

	// Interface ip, put in front of the port.
	// defaults to empty
	Interface string `json:"interface"`

	// semaphore stores ephemeral projects here
	TmpPath string `json:"tmp_path"`

	// cookie hashing & encryption
	CookieHash       string `json:"cookie_hash"`
	CookieEncryption string `json:"cookie_encryption"`
	// AccessKeyEncryption is BASE64 encoded byte array used
	// for encrypting and decrypting access keys stored in database.
	// Do not use it! Use method GetAccessKeyEncryption instead of it.
	AccessKeyEncryption string `json:"access_key_encryption"`

	// email alerting
	EmailSender   string `json:"email_sender"`
	EmailHost     string `json:"email_host"`
	EmailPort     string `json:"email_port"`
	EmailUsername string `json:"email_username"`
	EmailPassword string `json:"email_password"`

	// web host
	WebHost string `json:"web_host"`

	// ldap settings
	LdapBindDN       string       `json:"ldap_binddn"`
	LdapBindPassword string       `json:"ldap_bindpassword"`
	LdapServer       string       `json:"ldap_server"`
	LdapSearchDN     string       `json:"ldap_searchdn"`
	LdapSearchFilter string       `json:"ldap_searchfilter"`
	LdapMappings     ldapMappings `json:"ldap_mappings"`

	// external-auth/forward-auth
	ExternalAuth       bool `json:"external_auth"`
	ExternalAuthHeader struct {
		Username string `json:"username"`
		Group    string `json:"group"`
	} `json:"external_auth_header"`

	// telegram alerting
	TelegramChat  string `json:"telegram_chat"`
	TelegramToken string `json:"telegram_token"`

	// slack alerting
	SlackUrl string `json:"slack_url"`

	// task concurrency
	MaxParallelTasks int `json:"max_parallel_tasks"`

	// configType field ordering with bools at end reduces struct size
	// (maligned check)

	// feature switches
	EmailAlert    bool `json:"email_alert"`
	EmailSecure   bool `json:"email_secure"`
	TelegramAlert bool `json:"telegram_alert"`
	SlackAlert    bool `json:"slack_alert"`
	LdapEnable    bool `json:"ldap_enable"`
	LdapNeedTLS   bool `json:"ldap_needtls"`

	SshConfigPath string `json:"ssh_config_path"`

	DemoMode bool `json:"demo_mode"`

	GitClient string `json:"git_client"`
}

// Config exposes the application configuration storage for use in the application
var Config *ConfigType

// ToJSON returns a JSON string of the config
func (conf *ConfigType) ToJSON() ([]byte, error) {
	return json.MarshalIndent(&conf, " ", "\t")
}

func (conf *ConfigType) GetAccessKeyEncryption() string {
	ret := os.Getenv("SEMAPHORE_ACCESS_KEY_ENCRYPTION")

	if ret == "" {
		ret = conf.AccessKeyEncryption
	}

	return ret
}

// ConfigInit reads in cli flags, and switches actions appropriately on them
func ConfigInit(configPath string) {
	loadConfig(configPath)
	validateConfig()

	var encryption []byte

	hash, _ := base64.StdEncoding.DecodeString(Config.CookieHash)
	if len(Config.CookieEncryption) > 0 {
		encryption, _ = base64.StdEncoding.DecodeString(Config.CookieEncryption)
	}

	Cookie = securecookie.New(hash, encryption)
	WebHostURL, _ = url.Parse(Config.WebHost)
	if len(WebHostURL.String()) == 0 {
		WebHostURL = nil
	}
}

func loadConfig(configPath string) {
	if configPath == "" {
		configPath = os.Getenv("SEMAPHORE_CONFIG_PATH")
	}

	//If the configPath option has been set try to load and decode it
	//var usedPath string

	if configPath == "" {
		cwd, err := os.Getwd()
		exitOnConfigError(err)
		paths := []string{
			path.Join(cwd, "config.json"),
			"/usr/local/etc/semaphore/config.json",
		}
		for _, p := range paths {
			_, err = os.Stat(p)
			if err != nil {
				continue
			}
			var file *os.File
			file, err = os.Open(p)
			if err != nil {
				continue
			}
			decodeConfig(file)
			break
		}
		exitOnConfigError(err)
	} else {
		p := configPath
		file, err := os.Open(p)
		exitOnConfigError(err)
		decodeConfig(file)
	}
}

func validateConfig() {

	validatePort()
	validateExternalAuth()

	if len(Config.TmpPath) == 0 {
		Config.TmpPath = "/tmp/semaphore"
	}

	if Config.MaxParallelTasks < 1 {
		Config.MaxParallelTasks = 10
	}
}

func validateExternalAuth() {
	if !Config.ExternalAuth {
		return
	}

	if Config.ExternalAuthHeader.Username == "" {
		Config.ExternalAuthHeader.Username = "X-Username"
	}

	if Config.ExternalAuthHeader.Group == "" {
		Config.ExternalAuthHeader.Group = "X-Group"
	}
}

func validatePort() {

	//TODO - why do we do this only with this variable?
	if len(os.Getenv("PORT")) > 0 {
		Config.Port = ":" + os.Getenv("PORT")
	}
	if len(Config.Port) == 0 {
		Config.Port = ":3000"
	}
	if !strings.HasPrefix(Config.Port, ":") {
		Config.Port = ":" + Config.Port
	}
}

func exitOnConfigError(err error) {
	if err != nil {
		fmt.Println("Cannot Find configuration! Use --config parameter to point to a JSON file generated by `semaphore setup`.")
		os.Exit(1)
	}
}

func decodeConfig(file io.Reader) {
	if err := json.NewDecoder(file).Decode(&Config); err != nil {
		fmt.Println("Could not decode configuration!")
		panic(err)
	}
}

func mapToQueryString(m map[string]string) (str string) {
	for option, value := range m {
		if str != "" {
			str += "&"
		}
		str += option + "=" + value
	}
	if str != "" {
		str = "?" + str
	}
	return
}

// FindSemaphore looks in the PATH for the semaphore variable
// if not found it will attempt to find the absolute path of the first
// os argument, the semaphore command, and return it
func FindSemaphore() string {
	cmdPath, _ := exec.LookPath("semaphore") //nolint: gas

	if len(cmdPath) == 0 {
		cmdPath, _ = filepath.Abs(os.Args[0]) // nolint: gas
	}

	return cmdPath
}

func AnsibleVersion() string {
	bytes, err := exec.Command("ansible", "--version").Output()
	if err != nil {
		return ""
	}
	return string(bytes)
}

// CheckUpdate uses the GitHub client to check for new tags in the semaphore repo
func CheckUpdate() (updateAvailable *github.RepositoryRelease, err error) {
	// fetch releases
	gh := github.NewClient(nil)
	releases, _, err := gh.Repositories.ListReleases(context.TODO(), "ansible-semaphore", "semaphore", nil)
	if err != nil {
		return
	}

	updateAvailable = nil
	if (*releases[0].TagName)[1:] != Version {
		updateAvailable = releases[0]
	}

	return
}

// String returns dialect name for GORP.
func (d DbDriver) String() string {
	return string(d)
}

func (d *DbConfig) IsPresent() bool {
	return d.GetHostname() != ""
}

func (d *DbConfig) HasSupportMultipleDatabases() bool {
	return true
}

func (d *DbConfig) GetDbName() string {
	dbName := os.Getenv("SEMAPHORE_DB_NAME")
	if dbName != "" {
		return dbName
	}
	return d.DbName
}

func (d *DbConfig) GetUsername() string {
	username := os.Getenv("SEMAPHORE_DB_USER")
	if username != "" {
		return username
	}
	return d.Username
}

func (d *DbConfig) GetPassword() string {
	password := os.Getenv("SEMAPHORE_DB_PASS")
	if password != "" {
		return password
	}
	return d.Password
}

func (d *DbConfig) GetHostname() string {
	hostname := os.Getenv("SEMAPHORE_DB_HOST")
	if hostname != "" {
		return hostname
	}
	return d.Hostname
}

func (d *DbConfig) GetConnectionString(includeDbName bool) (connectionString string, err error) {
	dbName := d.GetDbName()
	dbUser := d.GetUsername()
	dbPass := d.GetPassword()
	dbHost := d.GetHostname()

	switch d.Dialect {
	case DbDriverBolt:
		connectionString = dbHost
	case DbDriverMySQL:
		if includeDbName {
			connectionString = fmt.Sprintf(
				"%s:%s@tcp(%s)/%s",
				dbUser,
				dbPass,
				dbHost,
				dbName)
		} else {
			connectionString = fmt.Sprintf(
				"%s:%s@tcp(%s)/",
				dbUser,
				dbPass,
				dbHost)
		}
		options := map[string]string{
			"parseTime":         "true",
			"interpolateParams": "true",
		}
		for v, k := range d.Options {
			options[v] = k
		}
		connectionString += mapToQueryString(options)
	case DbDriverPostgres:
		if includeDbName {
			connectionString = fmt.Sprintf(
				"postgres://%s:%s@%s/%s",
				dbUser,
				url.QueryEscape(dbPass),
				dbHost,
				dbName)
		} else {
			connectionString = fmt.Sprintf(
				"postgres://%s:%s@%s",
				dbUser,
				url.QueryEscape(dbPass),
				dbHost)
		}
		connectionString += mapToQueryString(d.Options)
	default:
		err = fmt.Errorf("unsupported database driver: %s", d.Dialect)
	}
	return
}

func (conf *ConfigType) PrintDbInfo() {
	dialect, err := conf.GetDialect()
	if err != nil {
		panic(err)
	}
	switch dialect {
	case DbDriverMySQL:
		fmt.Printf("MySQL %v@%v %v\n", conf.MySQL.GetUsername(), conf.MySQL.GetHostname(), conf.MySQL.GetDbName())
	case DbDriverBolt:
		fmt.Printf("BoltDB %v\n", conf.BoltDb.GetHostname())
	case DbDriverPostgres:
		fmt.Printf("Postgres %v@%v %v\n", conf.Postgres.GetUsername(), conf.Postgres.GetHostname(), conf.Postgres.GetDbName())
	default:
		panic(fmt.Errorf("database configuration not found"))
	}
}

func (conf *ConfigType) GetDialect() (dialect DbDriver, err error) {
	if conf.Dialect == "" {
		switch {
		case conf.MySQL.IsPresent():
			dialect = DbDriverMySQL
		case conf.BoltDb.IsPresent():
			dialect = DbDriverBolt
		case conf.Postgres.IsPresent():
			dialect = DbDriverPostgres
		default:
			err = errors.New("database configuration not found")
		}
		return
	}

	dialect = conf.Dialect
	return
}

func (conf *ConfigType) GetDBConfig() (dbConfig DbConfig, err error) {
	var dialect DbDriver
	dialect, err = conf.GetDialect()

	if err != nil {
		return
	}

	switch dialect {
	case DbDriverBolt:
		dbConfig = conf.BoltDb
	case DbDriverPostgres:
		dbConfig = conf.Postgres
	case DbDriverMySQL:
		dbConfig = conf.MySQL
	default:
		err = errors.New("database configuration not found")
	}

	dbConfig.Dialect = dialect

	return
}

// GenerateSecrets generates cookie secret during setup
func (conf *ConfigType) GenerateSecrets() {
	hash := securecookie.GenerateRandomKey(32)
	encryption := securecookie.GenerateRandomKey(32)
	accessKeyEncryption := securecookie.GenerateRandomKey(32)

	conf.CookieHash = base64.StdEncoding.EncodeToString(hash)
	conf.CookieEncryption = base64.StdEncoding.EncodeToString(encryption)
	conf.AccessKeyEncryption = base64.StdEncoding.EncodeToString(accessKeyEncryption)
}
