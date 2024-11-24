/*
Copyright © 2024 Alexey Shulutkov <github@shulutkov.ru>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const Gib int64 = 1024 * 1024 * 1024

func ToGib(v int) int64 { return int64(v) * Gib }

func In(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}

func AllInMap[T comparable](m1, m2 map[string]T) bool {
	if len(m2) == 0 {
		return true
	}

	for k, v := range m2 {
		if m1[k] != v {
			return false
		}
	}

	return true
}

func GetAllSshPublicKeys(homeDir string) ([]string, error) {
	sshDir := filepath.Join(homeDir, ".ssh")
	if _, err := os.Stat(sshDir); err != nil {
		return nil, err
	}

	dir, err := os.ReadDir(sshDir)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, entry := range dir {
		file := entry.Name()
		if entry.IsDir() ||
			file == "config" ||
			strings.HasPrefix(file, "known_hosts") {
			continue
		}

		var b []byte
		b, err = os.ReadFile(filepath.Join(sshDir, file))
		if err != nil {
			return nil, err
		}

		keyData := strings.TrimSpace(string(b))
		if strings.Contains(keyData, "PRIVATE KEY") {
			continue
		}

		keyParts := strings.Split(keyData, " ")
		if len(keyParts) < 2 {
			continue
		}

		keyBytes, err := base64.StdEncoding.DecodeString(keyParts[1])
		if err != nil {
			continue
		}

		if _, err = ssh.ParsePublicKey(keyBytes); err == nil {
			keys = append(keys, strings.Join(keyParts[:2], " "))
		}
	}

	return keys, nil
}
