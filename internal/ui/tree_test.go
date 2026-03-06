package ui

import (
	"strings"
	"testing"
)

func TestRenderTreeFull(t *testing.T) {
	deps := []DepNode{
		{
			Alias:  "git-operations",
			URL:    "github.com/example/git-skills@v1.0.0",
			Skills: []string{"git-commit", "git-branch", "git-operations"},
		},
		{
			Alias:  "style-guides",
			URL:    "github.com/other-org/style-skills@v2.3.1",
			Skills: []string{"python-style", "js-style"},
		},
	}

	localSkills := []string{"lint-check", "review-pr"}

	got := FormatTree("code-quality@1.0.0", localSkills, deps)

	expected := `code-quality@1.0.0
├── Local skills
│   ├── lint-check
│   └── review-pr
├── git-operations (github.com/example/git-skills@v1.0.0)
│   ├── git-commit
│   ├── git-branch
│   └── git-operations
└── style-guides (github.com/other-org/style-skills@v2.3.1)
    ├── python-style
    └── js-style
`

	if got != expected {
		t.Errorf("tree mismatch:\nGOT:\n%s\nEXPECTED:\n%s", got, expected)
	}
}

func TestRenderTreeNoLocalSkills(t *testing.T) {
	deps := []DepNode{
		{
			Alias:  "tools",
			URL:    "github.com/org/tools@v1.0.0",
			Skills: []string{"formatter"},
		},
	}

	got := FormatTree("my-package@0.1.0", nil, deps)

	expected := `my-package@0.1.0
└── tools (github.com/org/tools@v1.0.0)
    └── formatter
`

	if got != expected {
		t.Errorf("tree mismatch:\nGOT:\n%s\nEXPECTED:\n%s", got, expected)
	}
}

func TestRenderTreeLocalSkillsOnly(t *testing.T) {
	got := FormatTree("local-only@1.0.0", []string{"my-skill"}, nil)

	expected := `local-only@1.0.0
└── Local skills
    └── my-skill
`

	if got != expected {
		t.Errorf("tree mismatch:\nGOT:\n%s\nEXPECTED:\n%s", got, expected)
	}
}

func TestRenderTreeEmpty(t *testing.T) {
	got := FormatTree("empty@0.0.0", nil, nil)

	if !strings.HasPrefix(got, "empty@0.0.0\n") {
		t.Errorf("expected package name header, got: %q", got)
	}
}

func TestRenderTreeDeterministicOrder(t *testing.T) {
	deps := []DepNode{
		{Alias: "zebra", URL: "github.com/z/z@v1.0.0", Skills: []string{"z-skill"}},
		{Alias: "alpha", URL: "github.com/a/a@v1.0.0", Skills: []string{"a-skill"}},
	}

	got := FormatTree("pkg@1.0.0", nil, deps)

	alphaIdx := strings.Index(got, "alpha")
	zebraIdx := strings.Index(got, "zebra")
	if alphaIdx > zebraIdx {
		t.Error("expected alphabetical ordering: alpha before zebra")
	}
}
