package rule

type Registry struct {
	rules map[string]Rule
}

func NewRegistry() *Registry {
	return &Registry{rules: make(map[string]Rule)}
}

func (r *Registry) Register(rule Rule) {
	r.rules[rule.Name()] = rule
}

func (r *Registry) Get(name string) (Rule, bool) {
	rule, ok := r.rules[name]
	return rule, ok
}

func (r *Registry) All() map[string]Rule {
	return r.rules
}
