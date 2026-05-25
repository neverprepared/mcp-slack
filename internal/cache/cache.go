// Encrypted on-disk token cache and plaintext config store.
//
// Layout under $XDG_CONFIG_HOME/mcp-slack (default ~/.config/mcp-slack):
//
//	token.enc   — encrypted JSON blob (token, session, teamId, …)
//	config.json — plaintext non-secret config (ably_channel, …)
package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neverprepared/mcp-slack/internal/crypto"
	"github.com/neverprepared/mcp-slack/internal/secrets"
)

const (
	dirname    = "mcp-slack"
	tokenFile  = "token.enc"
	configFile = "config.json"
)

func ConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, dirname)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func tokenPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenFile), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// TokenCache is a thread-safe encrypted token store.
type TokenCache struct {
	mu    sync.RWMutex
	token map[string]any
}

func (c *TokenCache) Load() (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, err := tokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c.token = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pass, err := secrets.GetPassphrase()
	if err != nil {
		return nil, err
	}
	plaintext, err := crypto.Decrypt(string(data), pass)
	if err != nil {
		return nil, fmt.Errorf("token cache decrypt failed: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		return nil, err
	}
	c.token = payload
	return payload, nil
}

func (c *TokenCache) Save(token map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data := make(map[string]any, len(token)+1)
	for k, v := range token {
		data[k] = v
	}
	if _, ok := data["updated_at"]; !ok {
		data["updated_at"] = time.Now().Unix()
	}
	pass, err := secrets.GetPassphrase()
	if err != nil {
		return err
	}
	plaintext, err := json.Marshal(data)
	if err != nil {
		return err
	}
	encrypted, err := crypto.Encrypt(string(plaintext), pass)
	if err != nil {
		return err
	}
	path, err := tokenPath()
	if err != nil {
		return err
	}
	if err := atomicWrite(path, []byte(encrypted), 0600); err != nil {
		return err
	}
	c.token = data
	log.Printf("token cache updated (team=%v user=%v)", data["teamName"], data["userName"])
	return nil
}

func (c *TokenCache) Get() (map[string]any, error) {
	c.mu.RLock()
	if c.token != nil {
		t := c.token
		c.mu.RUnlock()
		return t, nil
	}
	c.mu.RUnlock()

	path, err := tokenPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return c.Load()
}

func (c *TokenCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	path, _ := tokenPath()
	os.Remove(path)
	c.token = nil
}

func LoadConfig() (map[string]any, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveConfig(cfg map[string]any) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0600)
}
