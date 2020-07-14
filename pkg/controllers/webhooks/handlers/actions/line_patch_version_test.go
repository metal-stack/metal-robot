package actions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLinePatches_Apply(t *testing.T) {
	testContent := `1
2
3`
	testResult := `1
a
3`
	tests := []struct {
		name    string
		input   string
		output  string
		r       LinePatches
		value   string
		wantErr bool
	}{
		{
			name:   "replace a line",
			input:  testContent,
			output: testResult,
			r: LinePatches{
				{Line: 2, ReplaceTemplate: "%s"},
			},
			value:   "a",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cn := func(file string) ([]byte, error) {
				return []byte(tt.input), nil
			}
			cw := func(file string, content []byte) error {
				if diff := cmp.Diff(string(content), tt.output); diff != "" {
					t.Errorf("Apply() diff: %v", diff)
				}
				return nil
			}
			if err := tt.r.Apply(cn, cw, tt.value); (err != nil) != tt.wantErr {
				t.Errorf("LinePatches.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
