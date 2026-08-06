// Runtime configuration wiring: the mode enum, its parser, and the line-width knob, all populated from the CLI's
// environment variables once and then read by every pass and renderer. Kept in its own package so both the pass tree
// and the render tree can depend on it without depending on each other.
package config

import (
	"fmt"
	"strconv"
)

// Mode selects how much reshaping the pipeline is allowed to do. The numeric order matches the "cumulative levels"
// contract: every higher mode is a strict superset of the passes run at lower modes.
type Mode int

const (
	ModeMinimal Mode = iota + 1
	ModeStandard
	ModeFull
	ModePedantic
)

// DefaultLineWidth is the fold/unfold display-column threshold used when FORMAT_YAML_LINE_WIDTH is unset. Chosen to
// match the most common editor guideline. Not exposed as a CLI flag - the env var keeps invocation ergonomics on par
// with FORMAT_YAML_MODE.
const DefaultLineWidth = 120

// MinTTYLineWidth is the floor for TTY-detected widths: below this, "wrapped" output is visually indistinguishable from
// unwrapped and just wastes vertical space, so a sub-20-column terminal falls back to this instead of the raw (width -
// 2). An explicit FORMAT_YAML_LINE_WIDTH is not clamped - if a user asks for 5, they get 5.
const MinTTYLineWidth = 20

// LineWidth is the active fold/unfold threshold, in display columns. Set from the environment at startup (see
// ParseLineWidth) so callers can override without recompiling; stays a single source of truth for the pass callbacks
// that measure candidate lines.
var LineWidth = DefaultLineWidth

// Maps FORMAT_YAML_MODE (or empty, meaning "use default") to the internal enum. Unknown values are a hard error rather
// than a silent fallback so a typo in a CI config surfaces immediately.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "standard":
		return ModeStandard, nil
	case "minimal":
		return ModeMinimal, nil
	case "full":
		return ModeFull, nil
	case "pedantic":
		return ModePedantic, nil
	}
	return 0, fmt.Errorf("unknown FORMAT_YAML_MODE %q; expected one of: minimal, standard, full, pedantic", s)
}

// Maps FORMAT_YAML_LINE_WIDTH (or empty, meaning "use default") to the fold/unfold display-column threshold. Rejects
// zero and negative values hard so an accidental `FORMAT_YAML_LINE_WIDTH=0` doesn't silently make every inline
// collection unflow at every mode.
func ParseLineWidth(s string) (int, error) {
	if s == "" {
		return DefaultLineWidth, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid FORMAT_YAML_LINE_WIDTH %q: expected a positive integer", s)
	}
	return n, nil
}
