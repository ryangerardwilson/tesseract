package quickshell

import "testing"

func TestValidateSessionName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: " root ", want: "root"},
		{name: "", wantErr: true},
		{name: "index", wantErr: true},
		{name: "bad:name", wantErr: true},
		{name: "bad\nname", wantErr: true},
	}
	for _, test := range tests {
		got, err := ValidateSessionName(test.name)
		if test.wantErr {
			if err == nil {
				t.Fatalf("ValidateSessionName(%q) succeeded", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ValidateSessionName(%q) failed: %v", test.name, err)
		}
		if got != test.want {
			t.Fatalf("ValidateSessionName(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}
