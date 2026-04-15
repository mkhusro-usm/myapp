package rule

import (
	"context"
	"fmt"
	"log"
	"strings"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

const (
	codeownersPath         = ".github/CODEOWNERS"
	codeownersBranchPrefix = "governance/codeowners"
	codeownersCommitMsg    = "chore: update CODEOWNERS per governance policy"
)

// CodeownersEntry represents a single required CODEOWNERS line.
type CodeownersEntry struct {
	Pattern string   `yaml:"pattern"`
	Owners  []string `yaml:"owners"`
}

func (e CodeownersEntry) line() string {
	return e.Pattern + " " + strings.Join(e.Owners, " ")
}

// CodeownersSettings holds the desired-state configuration for the CODEOWNERS rule.
type CodeownersSettings struct {
	Entries []CodeownersEntry `yaml:"entries"`
}

// Codeowners enforces that every repository has a .github/CODEOWNERS file
// containing the required entries defined in the governance config.
type Codeowners struct {
	client   *gh.Client
	settings CodeownersSettings
}

func NewCodeowners(client *gh.Client, settings CodeownersSettings) *Codeowners {
	return &Codeowners{client: client, settings: settings}
}

func (co *Codeowners) Name() string {
	return "codeowners"
}

// Evaluate checks whether the repository's CODEOWNERS file contains all required entries.
func (co *Codeowners) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository CODEOWNERS", repo.FullName())
	content, err := co.fetchContent(ctx, repo)
	if err != nil {
		return nil, err
	}

	return NewResult(co.Name(), repo.FullName(), co.check(content)), nil
}

// Apply creates a pull request that adds or updates the CODEOWNERS file
// so that all required entries are present.
func (co *Codeowners) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository CODEOWNERS", repo.FullName())
	content, err := co.fetchContent(ctx, repo)
	if err != nil {
		return nil, err
	}

	desiredContent := co.buildContent()

	// Already compliant — nothing to do.
	if content == desiredContent {
		return NewResult(co.Name(), repo.FullName(), nil), nil
	}

	prURL, err := co.client.ProposeFileChange(
		ctx, repo.Name, defaultBranch(repo),
		codeownersPath,
		codeownersBranchPrefix,
		codeownersCommitMsg,
		codeownersCommitMsg,
		"This PR was automatically created by the governance tool to ensure CODEOWNERS compliance.",
		[]byte(desiredContent),
	)
	if err != nil {
		return nil, err
	}

	r := NewResult(co.Name(), repo.FullName(), nil)
	r.Applied = true
	r.PullRequestURL = prURL

	return r, nil
}

// fetchContent retrieves the current CODEOWNERS file content.
// Returns an empty string (without error) when the file does not exist.
func (co *Codeowners) fetchContent(ctx context.Context, repo *gh.Repository) (string, error) {
	expression := defaultBranch(repo) + ":" + codeownersPath
	content, err := co.client.GetFileContent(ctx, repo.Name, expression)
	if err != nil {
		return "", fmt.Errorf("fetching CODEOWNERS for %s: %w", repo.FullName(), err)
	}

	return content, nil
}

// check returns violations for any missing or incomplete CODEOWNERS entries.
func (co *Codeowners) check(content string) []Violation {
	if content == "" {
		return []Violation{{
			Field:    "codeowners",
			Expected: "present",
			Actual:   "missing",
			Message:  fmt.Sprintf("CODEOWNERS file not found at %s", codeownersPath),
		}}
	}

	var violations []Violation
	for _, entry := range co.settings.Entries {
		violations = append(violations, checkEntry(content, entry)...)
	}

	return violations
}

// buildContent produces the desired CODEOWNERS file body.
// The config is the source of truth — the file is always written to match exactly.
func (co *Codeowners) buildContent() string {
	var lines []string
	lines = append(lines, "# CODEOWNERS — managed by governance tool")
	lines = append(lines, "# Do not edit manually; changes will be overwritten.")
	lines = append(lines, "")
	for _, entry := range co.settings.Entries {
		lines = append(lines, entry.line())
	}

	return strings.Join(lines, "\n") + "\n"
}

// checkEntry checks whether the exact CODEOWNERS line exists in the file.
// The config is the source of truth — the line must match exactly.
func checkEntry(content string, entry CodeownersEntry) []Violation {
	expected := entry.line()
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == expected {
			return nil
		}
	}

	return []Violation{{
		Field:    "codeowners-entry",
		Expected: expected,
		Actual:   "missing or mismatched",
		Message:  fmt.Sprintf("required entry %q not found or does not match exactly", entry.Pattern),
	}}
}
