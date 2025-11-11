package env

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// Load reads the dotenv file at path and sets the variables into the process
// environment. It returns the parsed map of key->value and any error.
// Lines starting with # are comments. Supports KEY=VAL and export KEY=VAL.
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// support optional leading export
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		// split on first '='
		idx := strings.Index(line, "=")
		if idx <= 0 {
			// ignore malformed lines
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// strip surrounding quotes if present
		if len(val) >= 2 {
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
		}
		// unescape simple \n and \r and \\
		val = strings.ReplaceAll(val, "\\n", "\n")
		val = strings.ReplaceAll(val, "\\r", "\r")
		val = strings.ReplaceAll(val, "\\\\", "\\")
		if key == "" {
			continue
		}
		// set into process env and map
		if err := os.Setenv(key, val); err != nil {
			return m, err
		}
		m[key] = val
	}
	if err := scanner.Err(); err != nil {
		return m, err
	}
	if len(m) == 0 {
		return m, errors.New("no variables loaded")
	}
	return m, nil
}
