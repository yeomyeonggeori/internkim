package desktop

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const LongestObservation = 16000

var ShallowerDepths = []int{3, 2, 1}

type Observation struct {
	Command           string   `json:"command"`
	Compact           []string `json:"compact"`
	Shallow           []string `json:"shallow"`
	SizeOptions       []string `json:"sizeOptions"`
	DepthOptions      []string `json:"depthOptions"`
	RegionOptions     []string `json:"regionOptions"`
	WindowOption      string   `json:"windowOption"`
	ApplicationOption string   `json:"applicationOption"`
	NarrowedBy        string   `json:"narrowedBy"`
}

func (call Call) program() Program {
	return Programs[call.Program]
}

func (call Call) Observes() bool {
	return call.Arguments[0] == call.program().Observation.Command
}

func (call Call) Acts() bool {
	return call.program().Acts(call.Arguments[0])
}

func IsTooLong(output string) bool {
	return utf8.RuneCountInString(output) > LongestObservation
}

func (observation Observation) sized(arguments []string) []string {
	if arguments[0] != observation.Command || observation.isSizedBy(arguments) {
		return arguments
	}
	return slices.Concat(arguments, observation.Compact)
}

func (observation Observation) isSizedBy(arguments []string) bool {
	chosen := slices.Concat(observation.SizeOptions, observation.DepthOptions, observation.RegionOptions)
	return slices.ContainsFunc(arguments, func(argument string) bool {
		return names(chosen, argument)
	})
}

func names(options []string, argument string) bool {
	option, _, _ := strings.Cut(argument, "=")
	return slices.Contains(options, option)
}

func (call Call) Shallower(depth int) Call {
	observation := call.program().Observation
	shallow := slices.Concat(observation.Shallow, []string{observation.DepthOptions[0], strconv.Itoa(depth)})
	return Call{Program: call.Program, Arguments: slices.Concat(observation.withoutSizing(call.Arguments), shallow)}
}

func (observation Observation) withoutSizing(arguments []string) []string {
	kept := make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if slices.Contains(observation.DepthOptions, argument) {
			index++
			continue
		}
		if names(observation.DepthOptions, argument) || slices.Contains(observation.SizeOptions, argument) {
			continue
		}
		kept = append(kept, argument)
	}
	return kept
}

type Scope struct {
	Window      string
	Application string
}

func (call Call) ObservationsAfter(envelope Envelope, scopeOfSnapshot func(snapshotID string) Scope) []Call {
	observation := call.program().Observation
	if observation.WindowOption == "" {
		return call.observations(nil)
	}
	scope := call.Scope(envelope)
	if scope == (Scope{}) {
		scope = scopeOfSnapshot(call.snapshotID())
	}
	return slices.Concat(
		call.observationsOf(observation.WindowOption, scope.Window),
		call.observationsOf(observation.ApplicationOption, scope.Application),
	)
}

func (call Call) observationsOf(option string, value string) []Call {
	if value == "" {
		return nil
	}
	return call.observations([]string{option, value})
}

func (call Call) observations(scope []string) []Call {
	observation, errorValue := NewCall(KindRead, call.Program, slices.Concat([]string{call.program().Observation.Command}, scope), "")
	if errorValue != nil {
		return nil
	}
	return []Call{observation}
}

func (call Call) Scope(envelope Envelope) Scope {
	observation := call.program().Observation
	if observation.WindowOption == "" {
		return Scope{}
	}
	return Scope{
		Window:      cmp.Or(envelope.Data.Window.ID, call.valueOf(observation.WindowOption)),
		Application: cmp.Or(call.valueOf(observation.ApplicationOption), envelope.Data.Application),
	}
}

func (call Call) valueOf(option string) string {
	for index, argument := range call.Arguments {
		if argument == option && index+1 < len(call.Arguments) {
			return call.Arguments[index+1]
		}
		if value, isJoined := strings.CutPrefix(argument, option+"="); isJoined {
			return value
		}
	}
	return ""
}

const (
	refPrefix            = "@"
	refSnapshotSeparator = ":"
)

func (call Call) snapshotID() string {
	for _, argument := range call.Arguments {
		reference, isReference := strings.CutPrefix(argument, refPrefix)
		if !isReference {
			continue
		}
		if snapshotID, _, isQualified := strings.Cut(reference, refSnapshotSeparator); isQualified {
			return snapshotID
		}
	}
	return ""
}
