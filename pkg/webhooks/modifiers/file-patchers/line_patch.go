package filepatchers

import (
	"fmt"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type LinePatch struct {
	file            string
	line            int
	replaceTemplate *string
}

func newLinePatch(rawConfig map[string]interface{}) (*LinePatch, error) {
	var typedConfig config.LinePatchConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	p := LinePatch{
		file:            typedConfig.File,
		line:            typedConfig.Line,
		replaceTemplate: typedConfig.ReplaceTemplate,
	}

	err = p.Validate()
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (p LinePatch) Apply(cr ContentReader, cw ContentWriter, newValue string) error {
	content, err := cr(p.file)
	if err != nil {
		return errors.Wrap(err, "error reading patch file")
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < p.line-1 {
		return fmt.Errorf("line %d does not exist in %s", p.line, p.file)
	}

	if p.replaceTemplate == nil {
		lines[p.line-1] = newValue
	} else {
		lines[p.line-1] = fmt.Sprintf(*p.replaceTemplate, newValue)
	}

	new := strings.Join(lines, "\n")

	err = cw(p.file, []byte(new))
	if err != nil {
		return errors.Wrap(err, "error writing patch file")
	}

	return nil
}

func (p LinePatch) Validate() error {
	if p.file == "" {
		return fmt.Errorf("file must be specified")
	}
	if p.line <= 0 {
		return fmt.Errorf("line cannot be 0 or lower, starts at 1")
	}
	return nil
}
