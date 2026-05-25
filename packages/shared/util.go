package shared

import (
	"encoding/json"
	"os"
	"strings"
)

func SanitizeID(id string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		":", "_",
	)
	return replacer.Replace(id)
}

func CloneDeep[T any](v *T) (*T, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var cloned T
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func CloneDeepSlice[T any](v []T) ([]T, error) {
	if len(v) == 0 {
		return []T{}, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var cloned []T
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func Contains[T comparable](slice []T, target T) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func SplitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range strings.Split(s, sep) {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
