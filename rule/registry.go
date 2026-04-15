package rule

// Registry holds all enabled governance rules, separated by scope.
type Registry struct {
	repoRules map[string]RepoRule
	orgRules  map[string]OrgRule
}

// NewRegistry creates an empty rule registry.
func NewRegistry() *Registry {
	return &Registry{
		repoRules: make(map[string]RepoRule),
		orgRules:  make(map[string]OrgRule),
	}
}

// RegisterRepoRule adds a repository-scoped rule.
func (r *Registry) RegisterRepoRule(rule RepoRule) {
	r.repoRules[rule.Name()] = rule
}

// RegisterOrgRule adds an organization-scoped rule.
func (r *Registry) RegisterOrgRule(rule OrgRule) {
	r.orgRules[rule.Name()] = rule
}

// RepoRules returns all registered repository-scoped rules.
func (r *Registry) RepoRules() map[string]RepoRule {
	return r.repoRules
}

// OrgRules returns all registered organization-scoped rules.
func (r *Registry) OrgRules() map[string]OrgRule {
	return r.orgRules
}
