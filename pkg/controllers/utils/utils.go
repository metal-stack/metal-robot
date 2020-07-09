package utils

import (
	"fmt"

	yamlconv "github.com/ghodss/yaml"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func SetYAML(data []byte, path string, value interface{}) ([]byte, error) {
	json, err := yamlconv.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}

	modified, err := sjson.Set(string(json), path, value)
	if err != nil {
		return nil, err
	}

	res, err := yamlconv.JSONToYAML([]byte(modified))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetYAML(data []byte, path string) (string, error) {
	json, err := yamlconv.YAMLToJSON(data)
	if err != nil {
		return "", err
	}

	res := gjson.Get(string(json), path)
	if err != nil {
		return "", err
	}

	if !res.Exists() {
		return "", fmt.Errorf("path not found in json: %v", path)
	}

	return res.String(), nil
}

func StringPtr(s string) *string {
	return &s
}

func BoolPtr(b bool) *bool {
	return &b
}
