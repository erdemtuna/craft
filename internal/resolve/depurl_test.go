package resolve

import "testing"

func TestParseDepURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *DepURL
		wantErr bool
	}{
		// Tag refs (existing)
		{
			name:  "standard github URL",
			input: "github.com/example/skills@v1.0.0",
			want: &DepURL{
				Raw:     "github.com/example/skills@v1.0.0",
				Host:    "github.com",
				Org:     "example",
				Repo:    "skills",
				Version: "1.0.0",
				RefType: RefTypeTag,
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
				RefType: RefTypeTag,
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
				RefType: RefTypeTag,
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
				RefType: RefTypeTag,
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
				RefType: RefTypeTag,
			},
		},
		// Commit SHA refs
		{
			name:  "7-char commit SHA",
			input: "github.com/acme/tools@abc1234",
			want: &DepURL{
				Raw:     "github.com/acme/tools@abc1234",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "abc1234",
				RefType: RefTypeCommit,
			},
		},
		{
			name:  "12-char commit SHA",
			input: "github.com/acme/tools@abc1234def567",
			want: &DepURL{
				Raw:     "github.com/acme/tools@abc1234def567",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "abc1234def567",
				RefType: RefTypeCommit,
			},
		},
		{
			name:  "full 40-char commit SHA",
			input: "github.com/acme/tools@abc1234def567890abc1234def567890abc1234d",
			want: &DepURL{
				Raw:     "github.com/acme/tools@abc1234def567890abc1234def567890abc1234d",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "abc1234def567890abc1234def567890abc1234d",
				RefType: RefTypeCommit,
			},
		},
		{
			name:  "uppercase hex SHA",
			input: "github.com/acme/tools@ABC1234DEF567",
			want: &DepURL{
				Raw:     "github.com/acme/tools@ABC1234DEF567",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "abc1234def567",
				RefType: RefTypeCommit,
			},
		},
		// Branch refs
		{
			name:  "branch main",
			input: "github.com/acme/tools@branch:main",
			want: &DepURL{
				Raw:     "github.com/acme/tools@branch:main",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "main",
				RefType: RefTypeBranch,
			},
		},
		{
			name:  "branch with slashes",
			input: "github.com/acme/tools@branch:feature/my-thing",
			want: &DepURL{
				Raw:     "github.com/acme/tools@branch:feature/my-thing",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "feature/my-thing",
				RefType: RefTypeBranch,
			},
		},
		{
			name:  "branch that looks like hex",
			input: "github.com/acme/tools@branch:deadbeef",
			want: &DepURL{
				Raw:     "github.com/acme/tools@branch:deadbeef",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "deadbeef",
				RefType: RefTypeBranch,
			},
		},
		{
			name:  "branch develop",
			input: "github.com/acme/tools@branch:develop",
			want: &DepURL{
				Raw:     "github.com/acme/tools@branch:develop",
				Host:    "github.com",
				Org:     "acme",
				Repo:    "tools",
				Ref:     "develop",
				RefType: RefTypeBranch,
			},
		},
		// Error cases
		{
			name:    "missing ref (no @)",
			input:   "github.com/org/repo",
			wantErr: true,
		},
		{
			name:    "missing v prefix (not valid for any ref type)",
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
		{
			name:    "SHA too short (5 chars)",
			input:   "github.com/org/repo@abc12",
			wantErr: true,
		},
		{
			name:    "SHA too short (6 chars)",
			input:   "github.com/org/repo@abc123",
			wantErr: true,
		},
		{
			name:    "SHA too long (65 chars)",
			input:   "github.com/org/repo@abc1234def5678901234567890123456789012345678901234567890abcdef123",
			wantErr: true,
		},
		{
			name:    "empty branch name",
			input:   "github.com/org/repo@branch:",
			wantErr: true,
		},
		{
			name:    "empty ref after @",
			input:   "github.com/org/repo@",
			wantErr: true,
		},
		{
			name:    "non-hex non-branch non-tag",
			input:   "github.com/org/repo@latest",
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
			if got.Raw != tt.want.Raw || got.Host != tt.want.Host || got.Org != tt.want.Org || got.Repo != tt.want.Repo || got.Version != tt.want.Version || got.Ref != tt.want.Ref || got.RefType != tt.want.RefType {
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

	if got := d.GitRef(); got != "v1.0.0" {
		t.Errorf("GitRef() = %q, want %q", got, "v1.0.0")
	}

	if got := d.RefString(); got != "v1.0.0" {
		t.Errorf("RefString() = %q, want %q", got, "v1.0.0")
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

func TestDepURLMethodsCommit(t *testing.T) {
	d, err := ParseDepURL("github.com/acme/tools@abc1234def")
	if err != nil {
		t.Fatalf("ParseDepURL error: %v", err)
	}

	if got := d.PackageIdentity(); got != "github.com/acme/tools" {
		t.Errorf("PackageIdentity() = %q, want %q", got, "github.com/acme/tools")
	}

	if got := d.GitRef(); got != "abc1234def" {
		t.Errorf("GitRef() = %q, want %q", got, "abc1234def")
	}

	if got := d.RefString(); got != "abc1234def" {
		t.Errorf("RefString() = %q, want %q", got, "abc1234def")
	}

	if got := d.RefType; got != RefTypeCommit {
		t.Errorf("RefType = %q, want %q", got, RefTypeCommit)
	}
}

func TestDepURLMethodsBranch(t *testing.T) {
	d, err := ParseDepURL("github.com/acme/tools@branch:main")
	if err != nil {
		t.Fatalf("ParseDepURL error: %v", err)
	}

	if got := d.PackageIdentity(); got != "github.com/acme/tools" {
		t.Errorf("PackageIdentity() = %q, want %q", got, "github.com/acme/tools")
	}

	// GitRef returns bare branch name (for fetcher.ResolveRef)
	if got := d.GitRef(); got != "main" {
		t.Errorf("GitRef() = %q, want %q", got, "main")
	}

	// RefString returns the URL form with branch: prefix
	if got := d.RefString(); got != "branch:main" {
		t.Errorf("RefString() = %q, want %q", got, "branch:main")
	}

	if got := d.RefType; got != RefTypeBranch {
		t.Errorf("RefType = %q, want %q", got, RefTypeBranch)
	}

	// String reconstructs the full URL
	if got := d.String(); got != "github.com/acme/tools@branch:main" {
		t.Errorf("String() = %q, want %q", got, "github.com/acme/tools@branch:main")
	}
}
