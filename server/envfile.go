package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// loadEnvFiles reads simple KEY=VALUE files for local development. Later files
// override earlier ones (.env.local over .env), and variables already set in
// the process environment always win, so containers and launch configs are
// unaffected. Missing files are skipped.
func loadEnvFiles(paths ...string) ([]string, error) {
	merged := map[string]string{}
	var loaded []string
	for _, path := range paths {
		values, err := readEnvFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return loaded, err
		}
		for k, v := range values {
			merged[k] = v
		}
		loaded = append(loaded, path)
	}
	for k, v := range merged {
		if _, set := os.LookupEnv(k); set {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return loaded, err
		}
	}
	return loaded, nil
}

func readEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")
		key, value, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i]) // trailing comment on an unquoted value
		}
		values[key] = value
	}
	return values, scanner.Err()
}
