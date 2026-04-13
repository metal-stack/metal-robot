package filepatchers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_setYAML(t *testing.T) {
	complex := `docker-images:
  metal-stack:
    control-plane:
      metal-api:
        name: metalstack/metal-api
        tag: v0.8.0
      metal-console:
        name: metalstack/metal-console
        tag: v0.4.0
`
	want := `docker-images:
  metal-stack:
    control-plane:
      metal-api:
        name: metalstack/metal-api
        tag: v0.8.1
      metal-console:
        name: metalstack/metal-console
        tag: v0.4.0
`
	type args struct {
		data  []byte
		path  string
		value any
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "replace something simple",
			args: args{
				data:  []byte("a: b"),
				path:  "a",
				value: "c",
			},
			want:    "a: c\n",
			wantErr: false,
		},
		{
			name: "replace release vector value",
			args: args{
				data:  []byte(complex),
				path:  "docker-images.metal-stack.control-plane.metal-api.tag",
				value: "v0.8.1",
			},
			want:    want,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setYAML(tt.args.data, tt.args.path, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("setYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(string(got), tt.want); diff != "" {
				t.Errorf("setYAML() diff: %v", diff)
			}
		})
	}
}

func TestYAMLPathVersionPatches_Apply(t *testing.T) {
	tpl := "http://server.io/%s.exe"

	tests := []struct {
		name     string
		p        YAMLPathPatch
		input    string
		output   string
		newValue string
		template string
		wantErr  bool
	}{
		{
			name:     "replace a path",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a"},
			newValue: "c",
			input:    "a: b",
			output:   "a: c\n",
			wantErr:  false,
		},
		{
			name:     "replace a path with version comparison",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", versionCompare: true},
			newValue: "v0.0.2",
			input:    "a: v0.0.1",
			output:   "a: v0.0.2\n",
			wantErr:  false,
		},
		{
			name:     "replace a path with version comparison when there was no semantic version before",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", versionCompare: true},
			newValue: "v0.0.2",
			input:    "a: bla",
			output:   "a: v0.0.2\n",
			wantErr:  false,
		},
		{
			name:     "replace a path with no prefix with version comparison",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", versionCompare: true},
			newValue: "0.0.2",
			input:    "a: 0.0.1",
			output:   "a: 0.0.2\n",
			wantErr:  false,
		},
		{
			name:     "change nothing on lower version",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", versionCompare: true},
			newValue: "0.0.1",
			input:    "a: v0.0.2",
			output:   "a: v0.0.2\n",
			wantErr:  false,
		},
		{
			name:     "replace a path with a template",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", template: &tpl},
			newValue: "c",
			input:    "a: http://server.io/b.exe",
			output:   "a: http://server.io/c.exe\n",
			wantErr:  false,
		},
		{
			name:     "replace with a template and version comparison",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", template: &tpl, versionCompare: true},
			newValue: "v0.0.2",
			input:    "a: http://server.io/v0.0.1.exe",
			output:   "a: http://server.io/v0.0.2.exe\n",
			wantErr:  false,
		},
		{
			name:     "change nothing on lower version with template",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", template: &tpl, versionCompare: true},
			newValue: "v0.0.2",
			input:    "a: http://server.io/v0.0.3.exe",
			output:   "a: http://server.io/v0.0.3.exe\n",
			wantErr:  false,
		},
		{
			name:     "replace with a template and version comparison when there is no semantic version",
			p:        YAMLPathPatch{file: "example.yaml", yamlPath: "a", template: &tpl, versionCompare: true},
			newValue: "0.0.2",
			input:    "a: http://server.io/bla.exe",
			output:   "a: http://server.io/0.0.2.exe\n",
			wantErr:  false,
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
			if err := tt.p.Apply(cn, cw, tt.newValue); (err != nil) != tt.wantErr {
				t.Errorf("YAMLPathVersionPatch.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
