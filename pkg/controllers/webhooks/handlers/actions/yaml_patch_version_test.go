package actions

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_SetYAML(t *testing.T) {
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
			got, err := SetYAML(tt.args.data, tt.args.path, tt.args.value)
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
