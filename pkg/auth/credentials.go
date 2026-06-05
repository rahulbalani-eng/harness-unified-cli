// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harness/harness-cli/pkg/hbase"
)

// custom "toml" parser for the single token field
// if this ever gets more complicated (or requires quoting/unquoting arbitrary strings) we should switch to a real TOML parser

const credentialsHeader = `# Harness credentials — contains sensitive tokens
# Do not share this file or open it on screen shares
# Manage credentials with: harness auth login / logout / status

`

// LoadCredentials reads ~/.harness/credentials and returns a map of profile → token.
// Returns an empty map if the file does not exist.
func LoadCredentials() (map[string]string, error) {
	path := hbase.GetCredentialsFilePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	return parseCredentials(string(data))
}

// parseCredentials parses a minimal TOML-like file: [section] / key = "value".
func parseCredentials(content string) (map[string]string, error) {
	result := map[string]string{}
	current := ""
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = line[1 : len(line)-1]
			continue
		}
		if current == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// unquote simple double-quoted strings
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
		}
		if k == "token" {
			result[current] = v
		}
	}
	return result, scanner.Err()
}

// SaveCredentials writes the credentials map to ~/.harness/credentials with 0600 perms.
func SaveCredentials(creds map[string]string) error {
	path := hbase.GetCredentialsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating credentials dir: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(credentialsHeader)
	for name, token := range creds {
		sb.WriteString("[")
		sb.WriteString(name)
		sb.WriteString("]\n")
		sb.WriteString("token = \"")
		sb.WriteString(token)
		sb.WriteString("\"\n\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}

// SetCredential adds or updates the token for the named profile and saves.
func SetCredential(profileName, token string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	creds[profileName] = token
	return SaveCredentials(creds)
}

// DeleteCredential removes the token for the named profile and saves.
// It is a no-op if the profile does not exist in the credentials file.
func DeleteCredential(profileName string) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	delete(creds, profileName)
	return SaveCredentials(creds)
}
