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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/gorilla/securecookie"
)

// Cookie is a runtime generated secure cookie used for authentication
var Cookie *securecookie.SecureCookie

// WebHostURL is the public route to the semaphore server
var WebHostURL *url.URL

const (
	DbDriverMySQL    = "mysql"
	DbDriverBolt     = "bolt"
	DbDriverPostgres = "postgres"
)

type DbConfig struct {
	Dialect string `json:"-"`

	Hostname string            `json:"host" env:"SEMAPHORE_DB_HOST"`
	Username string            `json:"user" env:"SEMAPHORE_DB_USER"`
	Password string            `json:"pass" env:"SEMAPHORE_DB_PASS"`
	DbName   string            `json:"name" env:"SEMAPHORE_DB"`
	Options  map[string]string `json:"options"`
}

type ldapMappings struct {
	DN   string `json:"dn"`
	Mail string `json:"mail"`
	UID  string `json:"uid"`
	CN   string `json:"cn"`
}

type oidcEndpoint struct {
	IssuerURL   string   `json:"issuer"`
	AuthURL     string   `json:"auth"`
	TokenURL    string   `json:"token"`
	UserInfoURL string   `json:"userinfo"`
	JWKSURL     string   `json:"jwks"`
	Algorithms  []string `json:"algorithms"`
}

type OidcProvider struct {
	ClientID         string       `json:"client_id"`
	ClientIDFile     string       `json:"client_id_file"`
	ClientSecret     string       `json:"client_secret"`
	ClientSecretFile string       `json:"client_secret_file"`
	RedirectURL      string       `json:"redirect_url"`
	Scopes           []string     `json:"scopes"`
	DisplayName      string       `json:"display_name"`
	Color            string       `json:"color"`
	Icon             string       `json:"icon"`
	AutoDiscovery    string       `json:"provider_url"`
	Endpoint         oidcEndpoint `json:"endpoint"`
	UsernameClaim    string       `json:"username_claim" default:"preferred_username"`
	NameClaim        string       `json:"name_claim" default:"preferred_username"`
	EmailClaim       string       `json:"email_claim" default:"email"`
	Order            int          `json:"order"`
}

const (
	// GoGitClientId is builtin Git client. It is not require external dependencies and is preferred.
	// Use it if you don't need external SSH authorization.
	GoGitClientId = "go_git"
	// CmdGitClientId is external Git client.
	// Default Git client. It is use external Git binary to clone repositories.
	CmdGitClientId = "cmd_git"
)

// // basic config validation using regex
// /* NOTE: other basic regex could be used:
//
//	ipv4: ^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$
//	ipv6: ^(?:[A-Fa-f0-9]{1,4}:|:){3,7}[A-Fa-f0-9]{1,4}$
//	domain: ^([a-zA-Z0-9]+(-[a-zA-Z0-9]+)*\.)+[a-zA-Z]{2,}$
//	path+filename: ^([\\/[a-zA-Z0-9_\\-${}:~]*]*\\/)?[a-zA-Z0-9\\.~_${}\\-:]*$
//	email address: ^(|.*@[A-Za-z0-9-\\.]*)$
//
// */

type RunnerConfig struct {
	RunnerID int    `json:"runner_id" env:"SEMAPHORE_RUNNER_ID"`
	Token    string `json:"token" env:"SEMAPHORE_RUNNER_TOKEN"`
}

type RunnerSettings struct {
	ApiURL            string `json:"api_url" env:"SEMAPHORE_RUNNER_API_URL"`
	RegistrationToken string `json:"registration_token" env:"SEMAPHORE_RUNNER_REGISTRATION_TOKEN"`
	ConfigFile        string `json:"config_file" env:"SEMAPHORE_RUNNER_CONFIG_FILE"`
	// OneOff indicates than runner runs only one job and exit
	OneOff bool `json:"one_off" env:"SEMAPHORE_RUNNER_ONE_OFF"`

	Webhook          string `json:"webhook" env:"SEMAPHORE_RUNNER_WEBHOOK"`
	MaxParallelTasks int    `json:"max_parallel_tasks" default:"1" env:"SEMAPHORE_RUNNER_MAX_PARALLEL_TASKS"`
}

// ConfigType mapping between Config and the json file that sets it
type ConfigType struct {
	MySQL    DbConfig `json:"mysql"`
	BoltDb   DbConfig `json:"bolt"`
	Postgres DbConfig `json:"postgres"`
	Dialect  string   `json:"dialect" default:"bolt" rule:"^mysql|bolt|postgres$" env:"SEMAPHORE_DB_DIALECT"`

	// Format `:port_num` eg, :3000
	// if : is missing it will be corrected
	Port string `json:"port" default:":3000" rule:"^:?([0-9]{1,5})$" env:"SEMAPHORE_PORT"`

	// Interface ip, put in front of the port.
	// defaults to empty
	Interface string `json:"interface" env:"SEMAPHORE_INTERFACE"`

	// semaphore stores ephemeral projects here
	TmpPath string `json:"tmp_path" default:"/tmp/semaphore" env:"SEMAPHORE_TMP_PATH"`

	// SshConfigPath is a path to the custom SSH config file.
	// Default path is ~/.ssh/config.
	SshConfigPath string `json:"ssh_config_path" env:"SEMAPHORE_TMP_PATH"`

	GitClientId string `json:"git_client" rule:"^go_git|cmd_git$" env:"SEMAPHORE_GIT_CLIENT" default:"cmd_git"`

	// web host
	WebHost string `json:"web_host" env:"SEMAPHORE_WEB_ROOT"`

	// cookie hashing & encryption
	CookieHash       string `json:"cookie_hash" env:"SEMAPHORE_COOKIE_HASH"`
	CookieEncryption string `json:"cookie_encryption" env:"SEMAPHORE_COOKIE_ENCRYPTION"`
	// AccessKeyEncryption is BASE64 encoded byte array used
	// for encrypting and decrypting access keys stored in database.
	AccessKeyEncryption string `json:"access_key_encryption" env:"SEMAPHORE_ACCESS_KEY_ENCRYPTION"`

	// email alerting
	EmailAlert    bool   `json:"email_alert" env:"SEMAPHORE_EMAIL_ALERT"`
	EmailSender   string `json:"email_sender" env:"SEMAPHORE_EMAIL_SENDER"`
	EmailHost     string `json:"email_host" env:"SEMAPHORE_EMAIL_HOST"`
	EmailPort     string `json:"email_port" rule:"^(|[0-9]{1,5})$" env:"SEMAPHORE_EMAIL_PORT"`
	EmailUsername string `json:"email_username" env:"SEMAPHORE_EMAIL_USERNAME"`
	EmailPassword string `json:"email_password" env:"SEMAPHORE_EMAIL_PASSWORD"`
	EmailSecure   bool   `json:"email_secure" env:"SEMAPHORE_EMAIL_SECURE"`

	// ldap settings
	LdapEnable       bool         `json:"ldap_enable" env:"SEMAPHORE_LDAP_ENABLE"`
	LdapBindDN       string       `json:"ldap_binddn" env:"SEMAPHORE_LDAP_BIND_DN"`
	LdapBindPassword string       `json:"ldap_bindpassword" env:"SEMAPHORE_LDAP_BIND_PASSWORD"`
	LdapServer       string       `json:"ldap_server" env:"SEMAPHORE_LDAP_SERVER"`
	LdapSearchDN     string       `json:"ldap_searchdn" env:"SEMAPHORE_LDAP_SEARCH_DN"`
	LdapSearchFilter string       `json:"ldap_searchfilter" env:"SEMAPHORE_LDAP_SEARCH_FILTER"`
	LdapMappings     ldapMappings `json:"ldap_mappings"`
	LdapNeedTLS      bool         `json:"ldap_needtls" env:"SEMAPHORE_LDAP_NEEDTLS"`

	TelegramChat      string `json:"telegram_chat" env:"SEMAPHORE_TELEGRAM_CHAT"`
	TelegramToken     string `json:"telegram_token" env:"SEMAPHORE_TELEGRAM_TOKEN"`
	SlackUrl          string `json:"slack_url" env:"SEMAPHORE_SLACK_URL"`
	RocketChatUrl     string `json:"rocketchat_url" env:"SEMAPHORE_ROCKETCHAT_URL"`
	MicrosoftTeamsUrl string `json:"microsoft_teams_url" env:"SEMAPHORE_MICROSOFT_TEAMS_URL"`

	// oidc settings
	OidcProviders map[string]OidcProvider `json:"oidc_providers"`

	MaxTaskDurationSec int `json:"max_task_duration_sec" env:"MAX_TASK_DURATION_SEC"`

	// task concurrency
	MaxParallelTasks int `json:"max_parallel_tasks" default:"10" rule:"^[0-9]{1,10}$" env:"SEMAPHORE_MAX_PARALLEL_TASKS"`

	// feature switches
	PasswordLoginDisable     bool `json:"password_login_disable" env:"SEMAPHORE_PASSWORD_LOGIN_DISABLED"`
	NonAdminCanCreateProject bool `json:"non_admin_can_create_project" env:"SEMAPHORE_NON_ADMIN_CAN_CREATE_PROJECT"`

	GlobalIntegrationAlias string `json:"global_integration_alias"`

	RunnerRegistrationToken string         `json:"runner_registration_token" env:"SEMAPHORE_RUNNER_REGISTRATION_TOKEN"`
	UseRemoteRunner         bool           `json:"use_remote_runner" env:"SEMAPHORE_USE_REMOTE_RUNNER"`
	Runner                  RunnerSettings `json:"runner"`
}

// Config exposes the application configuration storage for use in the application
var Config *ConfigType

// ToJSON returns a JSON string of the config
func (conf *ConfigType) ToJSON() ([]byte, error) {
	return json.MarshalIndent(&conf, " ", "\t")
}

func LoadRunnerSettings(path string) (config RunnerConfig, err error) {
	configFileExists := false

	if path != "" {
		_, err = os.Stat(path)

		if os.IsNotExist(err) {
			configFileExists = false
		} else if err != nil {
			return
		} else {
			configFileExists = true
		}
	}

	if configFileExists {

		var configBytes []byte
		configBytes, err = os.ReadFile(path)

		if err != nil {
			return
		}

		err = json.Unmarshal(configBytes, &config)

		if err != nil {
			return
		}

	}

	err = loadEnvironmentToObject(&config)

	if err != nil {
		return
	}

	err = loadDefaultsToObject(&config)

	return
}

// ConfigInit reads in cli flags, and switches actions appropriately on them
func ConfigInit(configPath string) {
	fmt.Println("Loading config")
	loadConfigFile(configPath)
	loadConfigEnvironment()
	loadConfigDefaults()

	fmt.Println("Validating config")
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

func loadConfigFile(configPath string) {
	if configPath == "" {
		configPath = os.Getenv("SEMAPHORE_CONFIG_PATH")
	}

	//If the configPath option has been set try to load and decode it
	//var usedPath string

	if configPath == "" {
		cwd, err := os.Getwd()
		exitOnConfigFileError(err)
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
		exitOnConfigFileError(err)
	} else {
		p := configPath
		file, err := os.Open(p)
		exitOnConfigFileError(err)
		decodeConfig(file)
	}
}

func loadDefaultsToObject(obj interface{}) error {
	var t = reflect.TypeOf(obj)
	var v = reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = reflect.Indirect(v)
	}

	for i := 0; i < t.NumField(); i++ {
		fieldInfo := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.IsZero() && fieldInfo.Type.Kind() != reflect.Struct && fieldInfo.Type.Kind() != reflect.Map {
			continue
		}

		if fieldInfo.Type.Kind() == reflect.Struct {
			err := loadDefaultsToObject(fieldValue.Addr().Interface())
			if err != nil {
				return err
			}
			continue
		} else if fieldInfo.Type.Kind() == reflect.Map {
			for _, key := range fieldValue.MapKeys() {
				val := fieldValue.MapIndex(key)

				if val.Type().Kind() != reflect.Struct {
					continue
				}

				newVal := reflect.New(val.Type())
				pointerValue := newVal.Elem()
				pointerValue.Set(val)

				err := loadDefaultsToObject(newVal.Interface())
				if err != nil {
					return err
				}

				fieldValue.SetMapIndex(key, newVal.Elem())
			}
			continue
		}

		defaultVar := fieldInfo.Tag.Get("default")
		if defaultVar == "" {
			continue
		}

		setConfigValue(fieldValue, defaultVar)
	}

	return nil
}

func loadConfigDefaults() {

	err := loadDefaultsToObject(Config)
	if err != nil {
		panic(err)
	}
}

func castStringToInt(value string) int {

	valueInt, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return valueInt

}

func castStringToBool(value string) bool {

	var valueBool bool
	if value == "1" || strings.ToLower(value) == "true" || strings.ToLower(value) == "yes" {
		valueBool = true
	} else {
		valueBool = false
	}
	return valueBool

}

func setConfigValue(attribute reflect.Value, value interface{}) {

	if attribute.IsValid() {
		switch attribute.Kind() {
		case reflect.Int:
			if reflect.ValueOf(value).Kind() != reflect.Int {
				value = castStringToInt(fmt.Sprintf("%v", reflect.ValueOf(value)))
			}
		case reflect.Bool:
			if reflect.ValueOf(value).Kind() != reflect.Bool {
				value = castStringToBool(fmt.Sprintf("%v", reflect.ValueOf(value)))
			}
		}
		attribute.Set(reflect.ValueOf(value))
	} else {
		panic(fmt.Errorf("got non-existent config attribute"))
	}

}

func getConfigValue(path string) string {

	attribute := reflect.ValueOf(Config)
	nested_path := strings.Split(path, ".")

	for i, nested := range nested_path {
		attribute = reflect.Indirect(attribute).FieldByName(nested)
		lastDepth := len(nested_path) == i+1
		if !lastDepth && attribute.Kind() != reflect.Struct || lastDepth && attribute.Kind() == reflect.Invalid {
			panic(fmt.Errorf("got non-existent config attribute '%v'", path))
		}
	}

	return fmt.Sprintf("%v", attribute)
}

func validate(value interface{}) error {
	var t = reflect.TypeOf(value)
	var v = reflect.ValueOf(value)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = reflect.Indirect(v)
	}

	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		fieldValue := v.Field(i)

		rule := fieldType.Tag.Get("rule")
		if rule == "" {
			continue
		}

		var strVal string

		if fieldType.Type.Kind() == reflect.Int {
			strVal = strconv.FormatInt(fieldValue.Int(), 10)
		} else if fieldType.Type.Kind() == reflect.Uint {
			strVal = strconv.FormatUint(fieldValue.Uint(), 10)
		} else {
			strVal = fieldValue.String()
		}

		match, _ := regexp.MatchString(rule, strVal)

		if match {
			continue
		}

		fieldName := strings.ToLower(fieldType.Name)

		if strings.Contains(fieldName, "password") || strings.Contains(fieldName, "secret") || strings.Contains(fieldName, "key") {
			strVal = "***"
		}

		return fmt.Errorf(
			"value of field '%v' is not valid: %v (Must match regex: '%v')",
			fieldType.Name, strVal, rule,
		)
	}

	return nil
}

func validateConfig() {

	err := validate(Config)

	if err != nil {
		panic(err)
	}
}

func loadEnvironmentToObject(obj interface{}) error {
	var t = reflect.TypeOf(obj)
	var v = reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = reflect.Indirect(v)
	}

	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i)
		fieldValue := v.Field(i)

		if fieldType.Type.Kind() == reflect.Struct {
			err := loadEnvironmentToObject(fieldValue.Addr().Interface())
			if err != nil {
				return err
			}
			continue
		}

		envVar := fieldType.Tag.Get("env")
		if envVar == "" {
			continue
		}

		envValue, exists := os.LookupEnv(envVar)

		if !exists {
			continue
		}

		setConfigValue(fieldValue, envValue)
	}

	return nil
}

func loadConfigEnvironment() {
	err := loadEnvironmentToObject(Config)
	if err != nil {
		panic(err)
	}
}

func exitOnConfigError(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func exitOnConfigFileError(err error) {
	if err != nil {
		exitOnConfigError("Cannot Find configuration! Use --config parameter to point to a JSON file generated by `semaphore setup`.")
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

func (conf *ConfigType) GetDialect() (dialect string, err error) {
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
	var dialect string
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
