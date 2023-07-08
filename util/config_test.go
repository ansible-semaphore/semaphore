package util

import (
	"os"
	"testing"
)

func TestValidatePort(t *testing.T) {

	Config = new(ConfigType)
	Config.Port = ""
	validatePort()

	if Config.Port != ":3000" {
		t.Error("no port should get set to default")
	}

	Config.Port = "4000"
	validatePort()
	if Config.Port != ":4000" {
		t.Error("Port without : suffix should have it added")
	}

	os.Setenv("PORT", "5000")
	validatePort()
	if Config.Port != ":5000" {
		t.Error("Port value should be overwritten by env var, and it should be prefixed appropriately")
	}
}

func TestExternalAuthIsFalseByDefault(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		Config = new(ConfigType)
		ConfigInit("fixture/empty-config.json")

		if Config.ExternalAuth {
			t.Error("External auth: should be disabled by default")
		}
	})

	t.Run("enabled", func(t *testing.T) {
		Config = new(ConfigType)
		ConfigInit("fixture/config.json")

		if !Config.ExternalAuth {
			t.Error("External auth: should be enabled")
		}

		if Config.ExternalAuthHeader.Username != "X-Username" {
			t.Errorf("External auth: wrong value for username: %s (expected: X-Username)", Config.ExternalAuthHeader.Username)
		}

		if Config.ExternalAuthHeader.Group != "X-Group" {
			t.Errorf("External auth: wrong value for group: %s (expected: X-Group)", Config.ExternalAuthHeader.Group)
		}
	})

	t.Run("full", func(t *testing.T) {
		Config = new(ConfigType)
		ConfigInit("fixture/config-external-auth.json")

		if !Config.ExternalAuth {
			t.Error("External auth: should be enabled")
		}

		if Config.ExternalAuthHeader.Username != "X-User" {
			t.Errorf("External auth: wrong value for username: %s (expected: X-User)", Config.ExternalAuthHeader.Username)
		}

		if Config.ExternalAuthHeader.Group != "X-Scope-OrgID" {
			t.Errorf("External auth: wrong value for group: %s (expected: X-Scope-OrgID)", Config.ExternalAuthHeader.Group)
		}
	})

	// t.Logf("Config: %#v", Config)
}
