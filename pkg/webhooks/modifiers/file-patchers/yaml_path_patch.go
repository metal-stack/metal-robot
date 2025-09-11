package filepatchers

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/utils"
	"github.com/mitchellh/mapstructure"

	yamlconv "sigs.k8s.io/yaml"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type YAMLPathPatch struct {
	file           string
	yamlPath       string
	template       *string
	versionCompare bool
}

func newYAMLPathPatch(rawConfig map[string]any) (*YAMLPathPatch, error) {
	var typedConfig config.YAMLPathPatchConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	p := YAMLPathPatch{
		file:           typedConfig.File,
		yamlPath:       typedConfig.YAMLPath,
		template:       typedConfig.Template,
		versionCompare: true,
	}

	if typedConfig.VersionCompare != nil {
		p.versionCompare = *typedConfig.VersionCompare
	}

	err = p.Validate()
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (p YAMLPathPatch) Apply(cn ContentReader, cw ContentWriter, newValue string) error {
	content, err := cn(p.file)
	if err != nil {
		return fmt.Errorf("error reading patch file %w", err)
	}

	if p.versionCompare {
		trimmedValue := strings.TrimPrefix(newValue, "v")

		newVersion, err := semver.NewVersion(trimmedValue)
		if err != nil {
			return err
		}

		old, err := GetYAML(content, p.yamlPath)
		if err != nil {
			return fmt.Errorf("error retrieving yaml path from file %w", err)
		}

		if p.template != nil {
			groups := utils.RegexCapture(utils.SemanticVersionMatcher, old)
			old = groups["full_match"]
		}

		old = strings.TrimPrefix(old, "v")
		oldVersion, err := semver.NewVersion(old)
		if err == nil {
			if !newVersion.GreaterThan(oldVersion) {
				return nil
			}
		}
	}

	if p.template != nil {
		newValue = fmt.Sprintf(*p.template, newValue)
	}

	content, err = setYAML(content, p.yamlPath, newValue)
	if err != nil {
		return err
	}

	err = cw(p.file, content)
	if err != nil {
		return fmt.Errorf("error writing patch file %w", err)
	}

	return nil
}

func setYAML(data []byte, path string, value any) ([]byte, error) {
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

	if !res.Exists() {
		return "", fmt.Errorf("path not found in json: %v", path)
	}

	return res.String(), nil
}

func (p YAMLPathPatch) Validate() error {
	if p.file == "" {
		return fmt.Errorf("file must be specified")
	}
	if p.yamlPath == "" {
		return fmt.Errorf("yaml-path must be specified")
	}
	return nil
}
