package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ansible-semaphore/semaphore/util"
)

const interactiveSetupBlurb = `
Hello! You will now be guided through a setup to:

1. Set up configuration for a MySQL/MariaDB database
2. Set up a path for your playbooks (auto-created)
3. Run database Migrations
4. Set up initial semaphore user & password

`

func InteractiveSetup(conf *util.ConfigType) {
	fmt.Print(interactiveSetupBlurb)

	dbPrompt := `What database to use:
   1 - MySQL
   2 - BoltDB
   3 - PostgreSQL
`

	var db int
	askValue(dbPrompt, "1", &db)

	switch db {
	case 1:
		conf.Dialect = util.DbDriverMySQL
		scanMySQL(conf)
	case 2:
		conf.Dialect = util.DbDriverBolt
		scanBoltDb(conf)
	case 3:
		conf.Dialect = util.DbDriverPostgres
		scanPostgres(conf)
	}

	defaultPlaybookPath := filepath.Join(os.TempDir(), "semaphore")
	askValue("Playbook path", defaultPlaybookPath, &conf.TmpPath)
	conf.TmpPath = filepath.Clean(conf.TmpPath)

	askValue("Public URL (optional, example: https://example.com/semaphore)", "", &conf.WebHost)

	askConfirmation("Enable email alerts?", false, &conf.EmailAlert)
	if conf.EmailAlert {
		askValue("Mail server host", "localhost", &conf.EmailHost)
		askValue("Mail server port", "25", &conf.EmailPort)
		askValue("Mail sender address", "semaphore@localhost", &conf.EmailSender)
	}

	var enable bool

	askConfirmation("Enable telegram alerts?", false, &enable)
	if enable {
		askValue("Telegram bot token (you can get it from @BotFather)", "", &conf.TelegramToken)
		askValue("Telegram chat ID", "", &conf.TelegramChat)
	}

	askConfirmation("Enable slack alerts?", false, &enable)
	if enable {
		askValue("Slack Webhook URL", "", &conf.SlackUrl)
	}

	askConfirmation("Enable Rocket.Chat alerts?", false, &enable)
	if enable {
		askValue("Rocket.Chat Webhook URL", "", &conf.RocketChatUrl)
	}

	askConfirmation("Enable Microsoft Team Channel alerts?", false, &enable)
	if enable {
		askValue("Microsoft Teams Webhook URL", "", &conf.MicrosoftTeamsUrl)
	}

	askConfirmation("Enable LDAP authentication?", false, &conf.LdapEnable)
	if conf.LdapEnable {
		askValue("LDAP server host", "localhost:389", &conf.LdapServer)
		askConfirmation("Enable LDAP TLS connection", false, &conf.LdapNeedTLS)
		askValue("LDAP DN for bind", "cn=user,ou=users,dc=example", &conf.LdapBindDN)
		askValue("Password for LDAP bind user", "pa55w0rd", &conf.LdapBindPassword)
		askValue("LDAP DN for user search", "ou=users,dc=example", &conf.LdapSearchDN)
		askValue("LDAP search filter", `(uid=%s)`, &conf.LdapSearchFilter)
		askValue("LDAP mapping for DN field", "dn", &conf.LdapMappings.DN)
		askValue("LDAP mapping for username field", "uid", &conf.LdapMappings.UID)
		askValue("LDAP mapping for full name field", "cn", &conf.LdapMappings.CN)
		askValue("LDAP mapping for email field", "mail", &conf.LdapMappings.Mail)
	}
}

func scanBoltDb(conf *util.ConfigType) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		workingDirectory = os.TempDir()
	}
	defaultBoltDBPath := filepath.Join(workingDirectory, "database.boltdb")
	askValue("db filename", defaultBoltDBPath, &conf.BoltDb.Hostname)
}

func scanMySQL(conf *util.ConfigType) {
	askValue("db Hostname", "127.0.0.1:3306", &conf.MySQL.Hostname)
	askValue("db User", "root", &conf.MySQL.Username)
	askValue("db Password", "", &conf.MySQL.Password)
	askValue("db Name", "semaphore", &conf.MySQL.DbName)
}

func scanPostgres(conf *util.ConfigType) {
	askValue("db Hostname", "127.0.0.1:5432", &conf.Postgres.Hostname)
	askValue("db User", "root", &conf.Postgres.Username)
	askValue("db Password", "", &conf.Postgres.Password)
	askValue("db Name", "semaphore", &conf.Postgres.DbName)
	if conf.Postgres.Options == nil {
		conf.Postgres.Options = make(map[string]string)
	}
	if _, exists := conf.Postgres.Options["sslmode"]; !exists {
		conf.Postgres.Options["sslmode"] = "disable"
	}
}

func scanErrorChecker(n int, err error) {
	if err != nil && err.Error() != "unexpected newline" {
		log.Warn("An input error occurred: " + err.Error())
	}
}

func SaveConfig(config *util.ConfigType) (configPath string) {
	configDirectory, err := os.Getwd()
	if err != nil {
		configDirectory, err = os.UserConfigDir()
		if err != nil {
			// Final fallback
			configDirectory = "/etc/semaphore"
		}
		configDirectory = filepath.Join(configDirectory, "semaphore")
	}
	askValue("Config output directory", configDirectory, &configDirectory)

	fmt.Printf("Running: mkdir -p %v..\n", configDirectory)

	if _, err = os.Stat(configDirectory); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(configDirectory, 0755)
		}
	}

	if err != nil {
		log.Panic("Could not create config directory: " + err.Error())
	}

	// Marshal config to json
	bytes, err := config.ToJSON()
	if err != nil {
		panic(err)
	}

	configPath = filepath.Join(configDirectory, "config.json")
	if err = os.WriteFile(configPath, bytes, 0644); err != nil {
		panic(err)
	}

	fmt.Printf("Configuration written to %v..\n", configPath)
	return
}

func AskConfigConfirmation(config *util.ConfigType) bool {
	bytes, err := config.ToJSON()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nGenerated configuration:\n %v\n\n", string(bytes))

	var correct bool
	askConfirmation("Is this correct?", true, &correct)
	return correct
}

func askValue(prompt string, defaultValue string, item interface{}) {
	// Print prompt with optional default value
	fmt.Print(prompt)
	if len(defaultValue) != 0 {
		fmt.Print(" (default " + defaultValue + ")")
	}
	fmt.Print(": ")

	_, _ = fmt.Sscanln(defaultValue, item)

	scanErrorChecker(fmt.Scanln(item))

	// Empty line after prompt
	fmt.Println("")
}

func askConfirmation(prompt string, defaultValue bool, item *bool) {
	defString := "yes"
	if !defaultValue {
		defString = "no"
	}

	fmt.Print(prompt + " (yes/no) (default " + defString + "): ")

	var answer string

	scanErrorChecker(fmt.Scanln(&answer))

	switch strings.ToLower(answer) {
	case "y", "yes":
		*item = true
	case "n", "no":
		*item = false
	default:
		*item = defaultValue
	}

	// Empty line after prompt
	fmt.Println("")
}
