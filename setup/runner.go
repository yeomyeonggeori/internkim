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

	// Title returns a human-readable progress line (called once per execution).
	Title func(ctx *Context) string

	// IsSatisfied returns true when the step's output is already present and
	// re-running is redundant. When the caller passes --force, this is ignored.
	// Safe to leave nil — treated as "never satisfied, always run".
	IsSatisfied func(ctx *Context) bool

	// Run performs the step. Returning an error halts the pipeline.
	Run func(ctx *Context) error
}

// Selector controls which steps execute.
type Selector struct {
	Only  []string // run exactly these (+ their unsatisfied deps)
	From  string   // run this step and everything after in registry order
	Skip  []string // exclude these, regardless of Only/From
	Force bool     // ignore IsSatisfied, run unconditionally
}

// Registry is the ordered list of steps the pipeline executes. Callers populate
// it (typically once at startup) and pass it to Run.
type Registry []Step

// byName returns the step with the given name, or nil.
func (r Registry) byName(name string) *Step {
	for i := range r {
		if r[i].Name == name {
			return &r[i]
		}
	}
	return nil
}

// resolve returns the ordered list of step names to execute for sel, including
// transitive unsatisfied dependencies.
func (r Registry) resolve(ctx *Context, sel Selector) ([]string, error) {
	skip := map[string]bool{}
	for _, s := range sel.Skip {
		skip[s] = true
	}

	// Validate referenced names.
	for _, n := range append(append([]string{}, sel.Only...), sel.Skip...) {
		if r.byName(n) == nil {
			return nil, fmt.Errorf("unknown step: %s", n)
		}
	}
	if sel.From != "" && r.byName(sel.From) == nil {
		return nil, fmt.Errorf("unknown step: %s", sel.From)
	}

	// Seed candidates.
	var seeds []string
	switch {
	case len(sel.Only) > 0:
		seeds = append(seeds, sel.Only...)
	case sel.From != "":
		started := false
		for _, s := range r {
			if s.Name == sel.From {
				started = true
			}
			if started {
				seeds = append(seeds, s.Name)
			}
		}
	default:
		for _, s := range r {
			seeds = append(seeds, s.Name)
		}
	}

	// Expand with unsatisfied deps (transitive). Deps run BEFORE the seeds
	// that need them. Registry order is preserved as tiebreaker.
	planned := map[string]bool{}
	var addWithDeps func(name string)
	addWithDeps = func(name string) {
		if skip[name] || planned[name] {
			return
		}
		step := r.byName(name)
		if step == nil {
			return
		}
		// Auto-include unsatisfied deps — unless caller explicitly used --only
		// and the dep appears satisfied, in which case we skip it entirely.
		for _, d := range step.Deps {
			if skip[d] {
				continue
			}
			depStep := r.byName(d)
			if depStep == nil {
				continue
			}
			if !sel.Force && depStep.IsSatisfied != nil && depStep.IsSatisfied(ctx) {
				continue // dep already done, skip
			}
			addWithDeps(d)
		}
		planned[name] = true
	}
	for _, s := range seeds {
		addWithDeps(s)
	}

	// Emit in registry order.
	var out []string
	for _, s := range r {
		if planned[s.Name] {
			out = append(out, s.Name)
		}
	}
	return out, nil
}

// Run executes the resolved step plan. Progress is printed to stdout.
func (r Registry) Run(ctx *Context, sel Selector) error {
	plan, err := r.resolve(ctx, sel)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Println("  no steps selected")
		return nil
	}
	for i, name := range plan {
		step := r.byName(name)
		title := step.Name
		if step.Title != nil {
			title = step.Title(ctx)
		}
		fmt.Printf("\n[%d/%d] %s  (%s)\n", i+1, len(plan), title, step.Name)
		if !sel.Force && step.IsSatisfied != nil && step.IsSatisfied(ctx) {
			fmt.Println("  이미 설정됨 — 건너뜀")
			continue
		}
		if step.Run == nil {
			continue
		}
		if err := step.Run(ctx); err != nil {
			return fmt.Errorf("step %s: %w", name, err)
		}
	}
	return nil
}

// ParseNames splits a comma-separated list, trimming whitespace and dropping empties.
func ParseNames(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
