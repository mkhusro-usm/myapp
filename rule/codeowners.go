package rule

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/mkhusro-usm/myapp/config"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

const (
	codeownersPath         = ".github/CODEOWNERS"
	codeownersBranchPrefix = "governance/codeowners"
)

// CodeownersEntry represents a single required CODEOWNERS line.
type CodeownersEntry struct {
	Pattern string   `yaml:"pattern"`
	Owners  []string `yaml:"owners"`
}

// line returns the CODEOWNERS file entry in standard format.
func (e CodeownersEntry) line() string {
	return e.Pattern + " " + strings.Join(e.Owners, " ")
}

// CodeownersSettings holds the desired-state configuration for the CODEOWNERS rule.
type CodeownersSettings struct {
	Entries []CodeownersEntry `yaml:"entries"`
}

// Codeowners enforces that every repository has a .github/CODEOWNERS file
// containing the required entries defined in the governance config.
// Per-repo overrides allow additional entries to be appended to the baseline.
type Codeowners struct {
	client    CodeownersClient
	settings  CodeownersSettings
	overrides map[string]CodeownersSettings // repo name → additional entries
}

// NewCodeowners creates a Codeowners rule with the given baseline settings.
// It extracts and parses any per-repo overrides for this rule from the raw
// overrides map. Override entries that fail to parse are logged and skipped.
func NewCodeowners(client CodeownersClient, settings CodeownersSettings, overrides map[string]config.RepoOverride) *Codeowners {
	return &Codeowners{
		client:    client,
		settings:  settings,
		overrides: parseCodeownersOverrides(settings, overrides),
	}
}

// Name returns the rule identifier.
func (co *Codeowners) Name() string {
	return "codeowners"
}

// Evaluate checks whether the repository's CODEOWNERS file contains all required entries.
func (co *Codeowners) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository CODEOWNERS", repo.FullName())

	effective := co.effectiveSettings(repo.Name)

	content, err := co.fetchContent(ctx, repo)
	if err != nil {
		return nil, err
	}

	var violations []Violation
	if content != buildContent(effective) {
		violations = co.check(content, effective)
	}

	return NewResult(co.Name(), repo.FullName(), violations), nil
}

// Apply creates a pull request that adds or updates the CODEOWNERS file
// so that all required entries are present.
func (co *Codeowners) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository CODEOWNERS", repo.FullName())

	effective := co.effectiveSettings(repo.Name)

	content, err := co.fetchContent(ctx, repo)
	if err != nil {
		return nil, err
	}

	desiredContent := buildContent(effective)

	// Already compliant — nothing to do.
	if content == desiredContent {
		return NewResult(co.Name(), repo.FullName(), nil), nil
	}

	prURL, err := co.client.CreateFileChangePR(
		ctx, repo.Name, defaultBranch(repo),
		codeownersBranchPrefix,
		"chore: update CODEOWNERS per governance policy",
		"This PR was automatically created by the governance tool to ensure CODEOWNERS compliance.",
		[]gh.FileChange{{
			Path:    codeownersPath,
			Content: []byte(desiredContent),
		}},
	)
	if err != nil {
		return nil, err
	}

	r := NewResult(co.Name(), repo.FullName(), nil)
	r.Applied = true
	r.PullRequestURL = prURL

	return r, nil
}

// parseCodeownersOverrides extracts and validates codeowners-specific overrides
// from the raw override map. Only entries with new patterns (not in the baseline)
// are accepted; conflicts are logged and skipped.
func parseCodeownersOverrides(baseline CodeownersSettings, raw map[string]config.RepoOverride) map[string]CodeownersSettings {
	result := make(map[string]CodeownersSettings)

	baselinePatterns := make(map[string]struct{}, len(baseline.Entries))
	for _, entry := range baseline.Entries {
		baselinePatterns[entry.Pattern] = struct{}{}
	}

	for repoName, repoOverride := range raw {
		overrideRule, exists := repoOverride.Rules["codeowners"]
		if !exists {
			continue
		}

		parsed, err := ParseSettings[CodeownersSettings](overrideRule.Settings)
		if err != nil {
			log.Printf("warning: failed to parse codeowners override for repo %q: %v (skipping)", repoName, err)
			continue
		}

		// Validate that no override entry redefines a baseline pattern.
		var validEntries []CodeownersEntry
		for _, entry := range parsed.Entries {
			if _, conflict := baselinePatterns[entry.Pattern]; conflict {
				log.Printf("warning: codeowners override for repo %q has pattern %q that conflicts with baseline (skipping entry)", repoName, entry.Pattern)
				continue
			}
			validEntries = append(validEntries, entry)
		}

		if len(validEntries) > 0 {
			result[repoName] = CodeownersSettings{Entries: validEntries}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// effectiveSettings returns the merged settings for a given repo: baseline + any overrides.
func (co *Codeowners) effectiveSettings(repoName string) CodeownersSettings {
	override, hasOverride := co.overrides[repoName]
	if !hasOverride || len(override.Entries) == 0 {
		return co.settings
	}

	log.Printf("[%s] applying codeowners override (%d additional entries)", repoName, len(override.Entries))

	merged := CodeownersSettings{
		Entries: make([]CodeownersEntry, 0, len(co.settings.Entries)+len(override.Entries)),
	}
	merged.Entries = append(merged.Entries, co.settings.Entries...)
	merged.Entries = append(merged.Entries, override.Entries...)

	return merged
}

// fetchContent retrieves the current CODEOWNERS file content.
// Returns an empty string (without error) when the file does not exist.
func (co *Codeowners) fetchContent(ctx context.Context, repo *gh.Repository) (string, error) {
	content, err := co.client.GetFileContent(ctx, repo.Name, defaultBranch(repo), codeownersPath)
	if err != nil {
		return "", fmt.Errorf("fetching CODEOWNERS for %s: %w", repo.FullName(), err)
	}

	return content, nil
}

// check returns violations for any missing or incomplete CODEOWNERS entries.
func (co *Codeowners) check(content string, effective CodeownersSettings) []Violation {
	if content == "" {
		return []Violation{{
			Field:    "codeowners",
			Expected: "present",
			Actual:   "missing",
			Message:  fmt.Sprintf("CODEOWNERS file not found at %s", codeownersPath),
		}}
	}

	existing := make(map[string]struct{})
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		existing[trimmed] = struct{}{}
	}

	desired := make(map[string]struct{}, len(effective.Entries))
	for _, entry := range effective.Entries {
		desired[entry.line()] = struct{}{}
	}

	var violations []Violation

	for _, entry := range effective.Entries {
		expected := entry.line()
		if _, found := existing[expected]; !found {
			violations = append(violations, Violation{
				Field:    "codeowners-entry",
				Expected: expected,
				Actual:   "missing or mismatched",
				Message:  fmt.Sprintf("required entry %q not found or does not match exactly", entry.Pattern),
			})
		}
	}

	for line := range existing {
		if _, expected := desired[line]; !expected {
			violations = append(violations, Violation{
				Field:    "codeowners-entry",
				Expected: "absent",
				Actual:   line,
				Message:  fmt.Sprintf("unexpected entry %q not defined in governance config", line),
			})
		}
	}

	return violations
}

// buildContent produces the desired CODEOWNERS file body.
// The config is the source of truth — the file is always written to match exactly.
func buildContent(settings CodeownersSettings) string {
	var lines []string
	lines = append(lines, "# CODEOWNERS — managed by governance tool")
	lines = append(lines, "# Do not edit manually; changes will be overwritten.")
	lines = append(lines, "")
	for _, entry := range settings.Entries {
		lines = append(lines, entry.line())
	}

	return strings.Join(lines, "\n") + "\n"
}
