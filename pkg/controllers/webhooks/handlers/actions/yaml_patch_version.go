package actions

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	yamlconv "github.com/ghodss/yaml"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type YAMLPathVersionPatch struct {
	File     string
	YAMLPath string
	Template string
}

type YAMLPathVersionPatches []YAMLPathVersionPatch

func (p YAMLPathVersionPatches) Apply(cn ContentReader, cw ContentWriter, version semver.Version, versionPrefix string) error {
	for _, patch := range p {
		content, err := cn(patch.File)
		if err != nil {
			return errors.Wrap(err, "error reading patch file")
		}

		value := versionPrefix + version.String()

		changed := false
		if patch.Template == "" {
			old, err := getYAML(content, patch.YAMLPath)
			if err != nil {
				return errors.Wrap(err, "error retrieving yaml path from file")
			}

			// check if old version is already a semantic version
			// if true, only replace if new version is greater than old
			old = strings.TrimPrefix(old, versionPrefix)
			oldVersion, err := semver.Parse(old)
			if err == nil && !version.GT(oldVersion) {
				continue
			}

			changed = true
		} else {
			value = fmt.Sprintf(patch.Template, value)
			changed = true
		}

		if !changed {
			return nil
		}

		content, err = setYAML(content, patch.YAMLPath, value)
		if err != nil {
			return err
		}

		err = cw(patch.File, content)
		if err != nil {
			return errors.Wrap(err, "error writing patch file")
		}
	}

	return nil
}

func setYAML(data []byte, path string, value interface{}) ([]byte, error) {
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

func getYAML(data []byte, path string) (string, error) {
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
