package filepatchers

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/pkg/errors"

	yamlconv "github.com/ghodss/yaml"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type YAMLPathPatch struct {
	config.YAMLPathPatchConfig
}

func (p YAMLPathPatch) Apply(cn ContentReader, cw ContentWriter, newValue string) error {
	content, err := cn(p.File)
	if err != nil {
		return errors.Wrap(err, "error reading patch file")
	}

	if p.VersionCompare {
		newValue = strings.TrimPrefix(newValue, "v")

		newVersion, err := semver.Parse(newValue)
		if err != nil {
			return err
		}

		old, err := getYAML(content, p.YAMLPath)
		if err != nil {
			return errors.Wrap(err, "error retrieving yaml path from file")
		}

		old = strings.TrimPrefix(old, "v")
		oldVersion, err := semver.Parse(old)
		if err != nil {
			return err
		}

		if !newVersion.GT(oldVersion) {
			return nil
		}

		newValue = "v" + newValue
	}

	if p.Template != nil {
		newValue = fmt.Sprintf(*p.Template, newValue)
	}

	content, err = setYAML(content, p.YAMLPath, newValue)
	if err != nil {
		return err
	}

	err = cw(p.File, content)
	if err != nil {
		return errors.Wrap(err, "error writing patch file")
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

func (p YAMLPathPatch) Validate() error {
	if p.File == "" {
		return fmt.Errorf("file must be specified")
	}
	if p.YAMLPath == "" {
		return fmt.Errorf("yaml-path must be specified")
	}
	if p.VersionCompare && p.Template != nil {
		return fmt.Errorf("when using version-compare, template can not be used")
	}
	return nil
}
