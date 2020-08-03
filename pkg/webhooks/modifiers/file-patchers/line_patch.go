package filepatchers

import (
	"fmt"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/pkg/errors"
)

type LinePatch struct {
	config.LinePatchConfig
}

func (p LinePatch) Apply(cr ContentReader, cw ContentWriter, newValue string) error {
	content, err := cr(p.File)
	if err != nil {
		return errors.Wrap(err, "error reading patch file")
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < p.Line-1 {
		return fmt.Errorf("line %d does not exist in %s", p.Line, p.File)
	}

	if p.ReplaceTemplate == nil {
		lines[p.Line-1] = newValue
	} else {
		lines[p.Line-1] = fmt.Sprintf(*p.ReplaceTemplate, newValue)
	}

	new := strings.Join(lines, "\n")

	err = cw(p.File, []byte(new))
	if err != nil {
		return errors.Wrap(err, "error writing patch file")
	}

	return nil
}

func (p LinePatch) Validate() error {
	if p.File == "" {
		return fmt.Errorf("file must be specified")
	}
	if p.Line == 0 {
		return fmt.Errorf("line cannot be 0, starts at 1")
	}
	return nil
}
