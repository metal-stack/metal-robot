package actions

import (
	"testing"

	"github.com/blang/semver"
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
		value interface{}
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
	tests := []struct {
		name     string
		p        YAMLPathVersionPatches
		input    string
		output   string
		version  semver.Version
		prefix   string
		template string
		wantErr  bool
	}{
		{
			name: "replace a path",
			p: YAMLPathVersionPatches{
				{File: "example.yaml", YAMLPath: "a"},
			},
			version: semver.MustParse("0.0.2"),
			input:   "a: v0.0.1",
			output:  "a: v0.0.2\n",
			prefix:  "v",
			wantErr: false,
		},
		{
			name: "replace a path without original prefix",
			p: YAMLPathVersionPatches{
				{File: "example.yaml", YAMLPath: "a"},
			},
			version: semver.MustParse("0.0.2"),
			input:   "a: 0.0.1",
			output:  "a: v0.0.2\n",
			prefix:  "v",
			wantErr: false,
		},
		{
			name: "replace a path with no prefix",
			p: YAMLPathVersionPatches{
				{File: "example.yaml", YAMLPath: "a"},
			},
			version: semver.MustParse("0.0.2"),
			input:   "a: 0.0.1",
			output:  "a: 0.0.2\n",
			prefix:  "",
			wantErr: false,
		},
		{
			name: "change nothing on lower version",
			p: YAMLPathVersionPatches{
				{File: "example.yaml", YAMLPath: "a"},
			},
			version: semver.MustParse("0.0.1"),
			input:   "a: v0.0.2",
			output:  "a: v0.0.2\n",
			prefix:  "v",
			wantErr: false,
		},
		{
			name: "replace with a template",
			p: YAMLPathVersionPatches{
				{File: "example.yaml", YAMLPath: "a", Template: "http://server.io/%s.exe"},
			},
			version: semver.MustParse("0.0.2"),
			input:   "a: something",
			output:  "a: http://server.io/v0.0.2.exe\n",
			prefix:  "v",
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
			if err := tt.p.Apply(cn, cw, tt.version, tt.prefix); (err != nil) != tt.wantErr {
				t.Errorf("YAMLPathVersionPatches.Apply() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
