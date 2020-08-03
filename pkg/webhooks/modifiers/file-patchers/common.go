package filepatchers

import (
	"fmt"

	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
)

const (
	LinePatchModifierName       string = "line-patch"
	YAMLPathVersionModifierName string = "yaml-path-version-patch"
)

type ContentReader func(file string) ([]byte, error)
type ContentWriter func(file string, content []byte) error

type Patcher interface {
	Apply(cr ContentReader, cw ContentWriter, newValue string) error
	Validate() error
}

func InitPatcher(c config.Modifier) (Patcher, error) {
	switch t := c.Type; t {
	case YAMLPathVersionModifierName:
		var patcher YAMLPathPatch
		err := mapstructure.Decode(c.Args, &patcher.YAMLPathPatchConfig)
		if err != nil {
			return nil, err
		}
		err = patcher.Validate()
		if err != nil {
			return nil, err
		}
		return patcher, nil
	case LinePatchModifierName:
		var patcher LinePatch
		err := mapstructure.Decode(c.Args, &patcher.LinePatchConfig)
		if err != nil {
			return nil, err
		}
		err = patcher.Validate()
		if err != nil {
			return nil, err
		}
		return patcher, nil
	default:
		return nil, fmt.Errorf("unsupported modifier type: %v", t)
	}
}
