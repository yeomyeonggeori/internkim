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
	Only  []string // run exactly these (+ their unsatisfied deps)
	From  string   // run this step and everything after in registry order
	Skip  []string // exclude these, regardless of Only/From
	Force bool     // ignore IsSatisfied on seed steps (not on auto-deps)
}

// Registry is the ordered list of steps the pipeline executes.
type Registry []Step

func (registry Registry) byName(name string) *Step {
	for index := range registry {
		if registry[index].Name == name {
			return &registry[index]
		}
	}
	return nil
}

func (registry Registry) resolve(context *Context, selector Selector) ([]string, error) {
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
			if dependencyStep.IsSatisfied != nil && dependencyStep.IsSatisfied(context) {
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

// Run executes the resolved step plan. Progress is printed to stdout.
func (registry Registry) Run(context *Context, selector Selector) error {
	plan, err := registry.resolve(context, selector)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Println("  no steps selected")
		return nil
	}
	for index, name := range plan {
		step := registry.byName(name)
		title := step.Name
		if step.Title != nil {
			title = step.Title(context)
		}
		fmt.Printf("\n[%d/%d] %s  (%s)\n", index+1, len(plan), title, step.Name)
		shouldForce := selector.Force && isExplicitlySeeded(name, selector)
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

func isExplicitlySeeded(name string, selector Selector) bool {
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

// ParseNames splits a comma-separated list, trimming whitespace and dropping empties.
func ParseNames(value string) []string {
	if value == "" {
		return nil
	}
	var names []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}
