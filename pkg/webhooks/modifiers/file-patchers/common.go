package filepatchers

import (
	"fmt"

	"github.com/metal-stack/metal-robot/pkg/config"
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
		return newYAMLPathPatch(c.Args)
	case LinePatchModifierName:
		return newLinePatch(c.Args)
	default:
		return nil, fmt.Errorf("unsupported modifier type: %v", t)
	}
}
