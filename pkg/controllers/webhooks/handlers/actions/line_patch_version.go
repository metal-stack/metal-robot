package actions

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type LinePatch struct {
	File            string
	Line            int
	ReplaceTemplate string
}

type LinePatches []LinePatch

func (r LinePatches) Apply(cn ContentReader, cw ContentWriter, value string) error {
	for _, patch := range r {
		content, err := cn(patch.File)
		if err != nil {
			return errors.Wrap(err, "error reading patch file")
		}

		lines := strings.Split(string(content), "\n")
		if len(lines) < patch.Line-1 {
			return fmt.Errorf("line %d does not exist in %s", patch.Line, patch.File)
		}

		lines[patch.Line-1] = fmt.Sprintf(patch.ReplaceTemplate, value)

		new := strings.Join(lines, "\n")

		err = cw(patch.File, []byte(new))
		if err != nil {
			return errors.Wrap(err, "error writing patch file")
		}
	}
	return nil
}
