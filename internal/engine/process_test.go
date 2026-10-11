package engine

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "simple",
			in:   "nginx -g daemon off;",
			want: []string{"nginx", "-g", "daemon", "off;"},
		},
		{
			name: "single quotes preserve spaces",
			in:   "nginx -g 'daemon off;'",
			want: []string{"nginx", "-g", "daemon off;"},
		},
		{
			name: "double quotes preserve spaces",
			in:   `sh -c "echo hello world"`,
			want: []string{"sh", "-c", "echo hello world"},
		},
		{
			name: "backslash escape",
			in:   `echo a\ b`,
			want: []string{"echo", "a b"},
		},
		{
			name: "escaped double quote inside arg",
			in:   `cmd --msg \"hello\"`,
			want: []string{"cmd", "--msg", `"hello"`},
		},
		{
			name: "quoted empty argument",
			in:   `cmd --name ""`,
			want: []string{"cmd", "--name", ""},
		},
		{
			name: "multiple spaces collapse",
			in:   "cmd    -a     -b",
			want: []string{"cmd", "-a", "-b"},
		},
		{
			name: "password with special chars in quotes",
			in:   `/ttyd -c 'user:p@ss word' -p 1024`,
			want: []string{"/ttyd", "-c", "user:p@ss word", "-p", "1024"},
		},
		{
			name: "malformed input falls back to fields",
			in:   "cmd 'unterminated",
			want: []string{"cmd", "'unterminated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCommand(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCommand(%q)\n  got  %#v\n  want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitCommandEmpty(t *testing.T) {
	if got := splitCommand(""); len(got) != 0 {
		t.Errorf("expected empty result, got %#v", got)
	}
	if got := splitCommand("   "); len(got) != 0 {
		t.Errorf("expected empty result for whitespace, got %#v", got)
	}
}
