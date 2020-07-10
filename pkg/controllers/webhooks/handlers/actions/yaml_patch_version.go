package actions

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	yamlconv "github.com/ghodss/yaml"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type YAMLPathPatch struct {
	Path     string
	Template string
}

type YAMLPathPatches []YAMLPathPatch

type YAMLVersionPatcher struct {
	Patches YAMLPathPatches
	Version semver.Version
	Content []byte
}

func (p YAMLVersionPatcher) Patch() ([]byte, error) {
	result := p.Content
	for _, patch := range p.Patches {
		old, err := GetYAML(result, patch.Path)
		if err != nil {
			return nil, errors.Wrap(err, "error retrieving path from release file")
		}

		value := "v" + p.Version.String()

		if patch.Template == "" {
			oldVersion, err := semver.Make(old[1:])
			if err != nil {
				return nil, err
			}

			if !p.Version.GT(oldVersion) {
				continue
			}
		} else {
			value = fmt.Sprintf(patch.Template, value)
		}

		result, err = SetYAML(result, patch.Path, value)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

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
