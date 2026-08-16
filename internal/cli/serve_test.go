package cli

import "testing"

func TestResolveServeRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		positional string
		flagOrEnv  string
		want       string
	}{
		{name: "positional wins", positional: "/proj", flagOrEnv: "/other", want: "/proj"},
		{name: "flag when no positional", positional: "", flagOrEnv: "/from-flag", want: "/from-flag"},
		{name: "default dot", positional: "", flagOrEnv: "", want: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveServeRoot(tt.positional, tt.flagOrEnv)
			if got != tt.want {
				t.Fatalf("resolveServeRoot(%q, %q) = %q, want %q", tt.positional, tt.flagOrEnv, got, tt.want)
			}
		})
	}
}
