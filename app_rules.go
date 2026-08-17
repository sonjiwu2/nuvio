package main

import "github.com/sonjiwu2/nuvio/internal/rules"

// ListRules returns every Organize rule, oldest first.
func (a *App) ListRules() ([]rules.Rule, error) {
	return a.rulesStore.List(a.ctx)
}

// AddRule creates a new Organize rule: files with the given extension
// would be previewed as moving to destinationFolder. This only persists
// the rule — see internal/rules' package doc for why Nuvio does not move
// any files yet.
func (a *App) AddRule(extension, destinationFolder string) (rules.Rule, error) {
	return a.rulesStore.Add(a.ctx, extension, destinationFolder)
}

// DeleteRule removes the rule with the given id.
func (a *App) DeleteRule(id string) error {
	return a.rulesStore.Delete(a.ctx, id)
}
