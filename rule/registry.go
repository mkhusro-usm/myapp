package rule

// Registry holds all enabled governance rules, keyed by name.
type Registry struct {
	rules map[string]Rule
}

// NewRegistry creates an empty rule registry.
func NewRegistry() *Registry {
	return &Registry{rules: make(map[string]Rule)}
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.rules[rule.Name()] = rule
}

// Get retrieves a rule by name. Returns false if not found.
func (r *Registry) Get(name string) (Rule, bool) {
	rule, ok := r.rules[name]
	return rule, ok
}

// All returns every registered rule.
func (r *Registry) All() map[string]Rule {
	return r.rules
}
