package resolve

import "testing"

func TestParseDepURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *DepURL
		wantErr bool
	}{
		{
			name:  "standard github URL",
			input: "github.com/example/skills@v1.0.0",
			want: &DepURL{
				Raw:     "github.com/example/skills@v1.0.0",
				Host:    "github.com",
				Org:     "example",
				Repo:    "skills",
				Version: "1.0.0",
			},
		},
		{
			name:  "multi-digit version",
			input: "github.com/org/repo@v10.20.30",
			want: &DepURL{
				Raw:     "github.com/org/repo@v10.20.30",
				Host:    "github.com",
				Org:     "org",
				Repo:    "repo",
				Version: "10.20.30",
			},
		},
		{
			name:  "dots in org name",
			input: "gitlab.example.io/my.org/my-repo@v0.1.0",
			want: &DepURL{
				Raw:     "gitlab.example.io/my.org/my-repo@v0.1.0",
				Host:    "gitlab.example.io",
				Org:     "my.org",
				Repo:    "my-repo",
				Version: "0.1.0",
			},
		},
		{
			name:  "underscores and dots in repo",
			input: "github.com/user/my_repo.go@v2.3.1",
			want: &DepURL{
				Raw:     "github.com/user/my_repo.go@v2.3.1",
				Host:    "github.com",
				Org:     "user",
				Repo:    "my_repo.go",
				Version: "2.3.1",
			},
		},
		{
			name:  "zero version",
			input: "github.com/a/b@v0.0.0",
			want: &DepURL{
				Raw:     "github.com/a/b@v0.0.0",
				Host:    "github.com",
				Org:     "a",
				Repo:    "b",
				Version: "0.0.0",
			},
		},
		{
			name:    "missing version",
			input:   "github.com/org/repo",
			wantErr: true,
		},
		{
			name:    "missing v prefix",
			input:   "github.com/org/repo@1.0.0",
			wantErr: true,
		},
		{
			name:    "leading zero in version",
			input:   "github.com/org/repo@v01.0.0",
			wantErr: true,
		},
		{
			name:    "pre-release version",
			input:   "github.com/org/repo@v1.0.0-beta",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing org",
			input:   "github.com/repo@v1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDepURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDepURL(%q) = %+v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDepURL(%q) error: %v", tt.input, err)
			}
			if got.Raw != tt.want.Raw || got.Host != tt.want.Host || got.Org != tt.want.Org || got.Repo != tt.want.Repo || got.Version != tt.want.Version {
				t.Errorf("ParseDepURL(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDepURLMethods(t *testing.T) {
	d, err := ParseDepURL("github.com/example/skills@v1.0.0")
	if err != nil {
		t.Fatalf("ParseDepURL error: %v", err)
	}

	if got := d.PackageIdentity(); got != "github.com/example/skills" {
		t.Errorf("PackageIdentity() = %q, want %q", got, "github.com/example/skills")
	}

	if got := d.GitTag(); got != "v1.0.0" {
		t.Errorf("GitTag() = %q, want %q", got, "v1.0.0")
	}

	if got := d.HTTPSURL(); got != "https://github.com/example/skills.git" {
		t.Errorf("HTTPSURL() = %q, want %q", got, "https://github.com/example/skills.git")
	}

	if got := d.SSHURL(); got != "git@github.com:example/skills.git" {
		t.Errorf("SSHURL() = %q, want %q", got, "git@github.com:example/skills.git")
	}

	if got := d.String(); got != "github.com/example/skills@v1.0.0" {
		t.Errorf("String() = %q, want %q", got, "github.com/example/skills@v1.0.0")
	}

	if got := d.WithVersion("v2.3.0"); got != "github.com/example/skills@v2.3.0" {
		t.Errorf("WithVersion(v2.3.0) = %q, want %q", got, "github.com/example/skills@v2.3.0")
	}

	if got := d.WithVersion("2.3.0"); got != "github.com/example/skills@v2.3.0" {
		t.Errorf("WithVersion(2.3.0) = %q, want %q", got, "github.com/example/skills@v2.3.0")
	}
}
