package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteProjectEnv keeps credentials out of command output and preserves unrelated settings.
func WriteProjectEnv(dir, filename string, values map[string]string, replace bool) error {
	if filename != ".env" && filename != ".env.local" {
		return errors.New("environment file must be .env or .env.local")
	}
	envPath := filepath.Join(dir, filename)
	old, err := readRegular(envPath)
	if err != nil {
		return err
	}
	updated, err := mergeProjectEnv(string(old), values, replace)
	if err != nil {
		return err
	}
	ignorePath := filepath.Join(dir, ".gitignore")
	ignore, err := readRegular(ignorePath)
	if err != nil {
		return err
	}
	// Append after existing rules so an earlier negation cannot expose this file.
	rule := "/" + filename
	ignoreLines := strings.Split(strings.TrimSpace(string(ignore)), "\n")
	if ignoreLines[len(ignoreLines)-1] != rule {
		ignore = []byte(strings.TrimRight(string(ignore), "\n") + "\n" + rule + "\n")
	}
	if err := writePrivateAtomic(ignorePath, ignore); err != nil {
		return err
	}
	return writePrivateAtomic(envPath, []byte(updated))
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("cannot inspect project configuration")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("project configuration must be a regular file, not a symlink")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("cannot read project configuration")
	}
	return data, nil
}

func mergeProjectEnv(old string, values map[string]string, replace bool) (string, error) {
	pending := make(map[string]string, len(values))
	for name, value := range values {
		if strings.ContainsAny(value, "\r\n\x00'\\") {
			return "", errors.New("invalid environment value")
		}
		pending[name] = value
	}
	lines := strings.Split(strings.TrimRight(old, "\n"), "\n")
	for i, line := range lines {
		name, found, err := envAssignment(line)
		if err != nil {
			return "", err
		}

		name = strings.TrimSpace(name)
		value, ours := values[name]
		if !found || !ours {
			continue
		}
		if !replace {
			return "", fmt.Errorf("%s already exists; pass --replace to update project credentials", name)
		}
		lines[i] = name + "='" + value + "'"
		delete(pending, name)
	}
	names := make([]string, 0, len(pending))
	for name := range pending {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lines = append(lines, name+"='"+pending[name]+"'")
	}
	return strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n", nil
}

func writePrivateAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".dwellir-config-*")
	if err != nil {
		return errors.New("cannot create project configuration")
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return errors.New("cannot write project configuration")
	}
	if err = file.Close(); err != nil {
		return errors.New("cannot close project configuration")
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return errors.New("cannot save project configuration")
	}
	return nil
}

// ValidateProjectEnv checks known conflicts before creating an account credential.
func ValidateProjectEnv(dir, filename string, replace bool) error {
	if filename != ".env" && filename != ".env.local" {
		return errors.New("environment file must be .env or .env.local")
	}
	old, err := readRegular(filepath.Join(dir, filename))
	if err != nil {
		return err
	}
	_, err = mergeProjectEnv(string(old), map[string]string{"DWELLIR_API_KEY": "", "DWELLIR_RPC_URL": "", "DWELLIR_WSS_URL": ""}, replace)
	if err != nil {
		return err
	}
	_, err = readRegular(filepath.Join(dir, ".gitignore"))
	return err
}

func envAssignment(line string) (string, bool, error) {
	name, value, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
	value = strings.TrimSpace(value)
	if found && len(value) > 0 && (value[0] == '\'' || value[0] == '"') && !hasClosingQuote(value) {
		return "", false, errors.New("multiline environment values require manual configuration")
	}
	return name, found, nil
}

func hasClosingQuote(value string) bool {
	for i := 1; i < len(value); i++ {
		if value[i] == '\\' {
			i++
			continue
		}
		if value[i] == value[0] {
			return true
		}
	}
	return false
}
