package ssh

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("admin", 2222, "/path/key", "pass")
	if client.User != "admin" {
		t.Errorf("expected user admin, got %s", client.User)
	}
	if client.Port != 2222 {
		t.Errorf("expected port 2222, got %d", client.Port)
	}
	if client.KeyPath != "/path/key" {
		t.Errorf("expected keypath /path/key, got %s", client.KeyPath)
	}
	if client.Password != "pass" {
		t.Errorf("expected password pass, got %s", client.Password)
	}
}

func TestNewClientDefaults(t *testing.T) {
	client := NewClient("", 0, "", "")
	if client.User != "postgres" {
		t.Errorf("expected default user postgres, got %s", client.User)
	}
	if client.Port != 22 {
		t.Errorf("expected default port 22, got %d", client.Port)
	}
}
