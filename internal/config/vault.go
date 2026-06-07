package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type VaultConfig struct {
	Address    string
	Token      string
	SecretPath string
}

func newVaultClient(address string) (*vault.Client, error) {
	cfg := vault.DefaultConfig()
	cfg.Address = address

	client, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault: создание клиента: %w", err)
	}

	return client, nil
}

func appRoleLogin(client *vault.Client, roleID, secretID string) error {
	data := map[string]any{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	secret, err := client.Logical().Write("auth/approle/login", data)
	if err != nil {
		return fmt.Errorf("vault: approle login: %w", err)
	}

	token, err := secret.TokenID()
	if err != nil {
		return fmt.Errorf("vault: нет токена в ответе approle: %w", err)
	}

	client.SetToken(token)

	return nil
}

func readSecretIDFromFile(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("vault: не могу прочитать %s: %v", path, err)
	}
	result := strings.TrimSpace(string(data))

	return result
}

func readVaultSecrets(client *vault.Client, path string) (map[string]string, error) {
	secret, err := client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("vault: чтение %s: %w", path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("vault: пустой ответ по пути %s", path)
	}

	var data map[string]any

	if d, ok := secret.Data["data"]; ok {
		// KV v2
		data, _ = d.(map[string]any)
	} else {
		// KV v1
		data = secret.Data
	}

	if data == nil {
		return nil, fmt.Errorf("vault: пустые секреты по пути %s", path)
	}

	result := make(map[string]string, len(data))
	for key, val := range data {
		if s, ok := val.(string); ok {
			result[key] = s
		}
	}

	return result, nil
}

func LoadFromVault(cfg VaultConfig) (map[string]string, error) {
	client, err := newVaultClient(cfg.Address)
	if err != nil {
		return nil, err
	}

	client.SetToken(cfg.Token)

	return readVaultSecrets(client, cfg.SecretPath)
}

func LoadFromVaultAppRole(address, roleID, secretID, path string) (map[string]string, error) {
	client, err := newVaultClient(address)
	if err != nil {
		return nil, err
	}

	if err := appRoleLogin(client, roleID, secretID); err != nil {
		return nil, err
	}

	return readVaultSecrets(client, path)
}
