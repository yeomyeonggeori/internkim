// Package setup defines the step-based provisioning pipeline.
//
// Each provisioning action is a Step with a stable Name, optional Deps, and
// an IsSatisfied fast-path for skipping work already done. Run() executes a
// selected subset of steps, auto-running unsatisfied dependencies.
package setup

import (
	"fmt"
	"strings"
)

// Step is one unit of provisioning work.
type Step struct {
	// Name is the stable identifier used by --only/--from/--skip.
	Name string

	// Deps are names of steps whose output this step requires.
	Deps []string

	// ForceDeps are dependencies that must also re-run when this step is an
	// explicitly forced seed. Other dependencies still honour IsSatisfied.
	ForceDeps []string

	// Title returns a human-readable progress line.
	Title func(context *Context) string

	// IsSatisfied returns true when the step's output is already present and
	// re-running is redundant. When the caller passes --force, this is ignored.
	// Safe to leave nil — treated as "never satisfied, always run".
	IsSatisfied func(context *Context) bool

	// Run is the SSH backend body (applied to a running board).
	Run func(context *Context) error

	// RunSD is the SD staging backend body (writes to the boot partition
	// for firstboot to apply on next boot). Nil → step not supported on SD.
	RunSD func(context *Context) error
}

// body returns the appropriate Run body for context.Backend, or nil if the
// step has no implementation for that backend.
func (step Step) body(context *Context) func(*Context) error {
	switch context.Backend {
	case BackendSD:
		return step.RunSD
	default:
		return step.Run
	}
}

// Selector controls which steps execute.
type Selector struct {
	Only     []string // run exactly these (+ their unsatisfied deps)
	From     string   // run this step and everything after in registry order
	Skip     []string // exclude these, regardless of Only/From
	Force    bool     // ignore IsSatisfied on seed steps (not on auto-deps)
	ForceAll bool     // ignore IsSatisfied on every planned step
	DryRun   bool     // print plan without executing
}

// Registry is the ordered list of steps the pipeline executes.
type Registry []Step

type planEntry struct {
	name   string
	status string
	reason string
}

func (registry Registry) byName(name string) *Step {
	name = canonicalStepName(name)
	for index := range registry {
		if registry[index].Name == name {
			return &registry[index]
		}
	}
	return nil
}

func (registry Registry) resolve(context *Context, selector Selector) ([]string, error) {
	selector.Only = canonicalStepNames(selector.Only)
	selector.Skip = canonicalStepNames(selector.Skip)
	selector.From = canonicalStepName(selector.From)
	skippedSteps := map[string]bool{}
	for _, name := range selector.Skip {
		skippedSteps[name] = true
	}

	for _, name := range append(append([]string{}, selector.Only...), selector.Skip...) {
		if registry.byName(name) == nil {
			return nil, fmt.Errorf("unknown step: %s", name)
		}
	}
	if selector.From != "" && registry.byName(selector.From) == nil {
		return nil, fmt.Errorf("unknown step: %s", selector.From)
	}

	var seeds []string
	switch {
	case len(selector.Only) > 0:
		seeds = append(seeds, selector.Only...)
	case selector.From != "":
		started := false
		for _, step := range registry {
			if step.Name == selector.From {
				started = true
			}
			if started {
				seeds = append(seeds, step.Name)
			}
		}
	default:
		for _, step := range registry {
			seeds = append(seeds, step.Name)
		}
	}

	// --force applies only to explicitly-seeded steps. Auto-included deps
	// always honour IsSatisfied so a rebuild of one step doesn't cascade
	// and re-do work that is already done.
	plannedSteps := map[string]bool{}
	var includeStepAndDeps func(name string)
	includeStepAndDeps = func(name string) {
		if skippedSteps[name] || plannedSteps[name] {
			return
		}
		step := registry.byName(name)
		if step == nil {
			return
		}
		for _, dependencyName := range step.Deps {
			if skippedSteps[dependencyName] {
				continue
			}
			dependencyStep := registry.byName(dependencyName)
			if dependencyStep == nil {
				continue
			}
			shouldForceDependency := selector.Force && isExplicitlySeeded(name, selector) && containsStepName(step.ForceDeps, dependencyName)
			if !selector.ForceAll && !shouldForceDependency && dependencyStep.IsSatisfied != nil && dependencyStep.IsSatisfied(context) {
				continue
			}
			includeStepAndDeps(dependencyName)
		}
		plannedSteps[name] = true
	}
	for _, seedName := range seeds {
		includeStepAndDeps(seedName)
	}

	var orderedPlan []string
	for _, step := range registry {
		if plannedSteps[step.Name] {
			orderedPlan = append(orderedPlan, step.Name)
		}
	}
	return orderedPlan, nil
}

func (registry Registry) plan(context *Context, selector Selector) ([]planEntry, error) {
	plan, err := registry.resolve(context, selector)
	if err != nil {
		return nil, err
	}

	plannedSteps := map[string]bool{}
	for _, name := range plan {
		plannedSteps[name] = true
	}
	skippedSteps := map[string]bool{}
	for _, name := range selector.Skip {
		skippedSteps[name] = true
	}

	var entries []planEntry
	for _, step := range registry {
		switch {
		case skippedSteps[step.Name]:
			entries = append(entries, planEntry{name: step.Name, status: "skip", reason: "requested"})
		case !plannedSteps[step.Name]:
			if step.IsSatisfied != nil && step.IsSatisfied(context) {
				entries = append(entries, planEntry{name: step.Name, status: "skip", reason: "satisfied"})
			}
		default:
			if step.body(context) == nil {
				entries = append(entries, planEntry{name: step.Name, status: "unsupported", reason: string(context.Backend)})
				continue
			}
			shouldForce := registry.shouldForceStep(step.Name, selector)
			if !shouldForce && step.IsSatisfied != nil && step.IsSatisfied(context) {
				entries = append(entries, planEntry{name: step.Name, status: "skip", reason: "satisfied"})
				continue
			}
			entries = append(entries, planEntry{name: step.Name, status: "run"})
		}
	}
	return entries, nil
}

func (registry Registry) PrintPlan(context *Context, selector Selector) error {
	entries, err := registry.plan(context, selector)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("  no steps selected")
		return nil
	}
	for _, entry := range entries {
		if entry.reason == "" {
			fmt.Printf("  %-12s %s\n", entry.name, entry.status)
			continue
		}
		fmt.Printf("  %-12s %s: %s\n", entry.name, entry.status, entry.reason)
	}
	return nil
}

func (registry Registry) PrintSteps() {
	for index, step := range registry {
		var backends []string
		if step.Run != nil {
			backends = append(backends, string(BackendSSH))
		}
		if step.RunSD != nil {
			backends = append(backends, string(BackendSD))
		}
		if len(backends) == 0 {
			backends = append(backends, "none")
		}
		fmt.Printf("  %2d. %-12s %s\n", index+1, step.Name, strings.Join(backends, ","))
	}
}

// Run executes the resolved step plan. Progress is printed to stdout.
func (registry Registry) Run(context *Context, selector Selector) error {
	if selector.DryRun {
		return registry.PrintPlan(context, selector)
	}
	plan, err := registry.resolve(context, selector)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Println("  no steps selected")
		return nil
	}
	previousPlannedSteps := context.PlannedSteps
	context.PlannedSteps = plannedStepSet(plan)
	defer func() {
		context.PlannedSteps = previousPlannedSteps
	}()
	for index, name := range plan {
		step := registry.byName(name)
		title := step.Name
		if step.Title != nil {
			title = step.Title(context)
		}
		fmt.Printf("\n[%d/%d] %s  (%s)\n", index+1, len(plan), title, step.Name)
		shouldForce := registry.shouldForceStep(name, selector)
		if !shouldForce && step.IsSatisfied != nil && step.IsSatisfied(context) {
			fmt.Println("  이미 설정됨 — 건너뜀")
			continue
		}
		body := step.body(context)
		if body == nil {
			if isExplicitlySeeded(name, selector) {
				return fmt.Errorf("step %s: %w (%s)", name, ErrUnsupportedBackend, context.Backend)
			}
			continue
		}
		if err := body(context); err != nil {
			return fmt.Errorf("step %s: %w", name, err)
		}
	}
	return nil
}

func (registry Registry) shouldForceStep(name string, selector Selector) bool {
	if selector.ForceAll {
		return true
	}
	if !selector.Force {
		return false
	}
	if isExplicitlySeeded(name, selector) {
		return true
	}
	for _, seedName := range selector.Only {
		step := registry.byName(seedName)
		if step != nil && containsStepName(step.ForceDeps, name) {
			return true
		}
	}
	if selector.From != "" {
		for _, step := range registry {
			if step.Name == selector.From {
				return containsStepName(step.ForceDeps, name)
			}
		}
	}
	return false
}

func isExplicitlySeeded(name string, selector Selector) bool {
	if len(selector.Only) == 0 && selector.From == "" {
		return true
	}
	for _, explicitName := range selector.Only {
		if explicitName == name {
			return true
		}
	}
	if selector.From == name {
		return true
	}
	return false
}

func plannedStepSet(plan []string) map[string]bool {
	plannedSteps := map[string]bool{}
	for _, name := range plan {
		plannedSteps[name] = true
	}
	return plannedSteps
}

func containsStepName(names []string, expectedName string) bool {
	for _, name := range names {
		if canonicalStepName(name) == expectedName {
			return true
		}
	}
	return false
}

// ParseNames splits a comma-separated list, trimming whitespace and dropping empties.
func ParseNames(value string) []string {
	if value == "" {
		return nil
	}
	var names []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			names = append(names, canonicalStepName(trimmed))
		}
	}
	return names
}

func canonicalStepNames(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if canonicalName := canonicalStepName(name); canonicalName != "" {
			result = append(result, canonicalName)
		}
	}
	return result
}

func canonicalStepName(name string) string {
	switch strings.TrimSpace(name) {
	case "admin-web":
		return "web"
	default:
		return strings.TrimSpace(name)
	}
}
