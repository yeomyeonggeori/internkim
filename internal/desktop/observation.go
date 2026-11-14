package desktop

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const LongestObservation = 16000

var ShallowerDepths = []int{3, 2, 1}

type Observation struct {
	Command       string   `json:"command"`
	Compact       []string `json:"compact"`
	Shallow       []string `json:"shallow"`
	SizeOptions   []string `json:"sizeOptions"`
	DepthOptions  []string `json:"depthOptions"`
	RegionOptions []string `json:"regionOptions"`
	NarrowedBy    string   `json:"narrowedBy"`
}

func (call Call) program() Program {
	return Programs[call.Program]
}

func (call Call) Observes() bool {
	return call.Arguments[0] == call.program().Observation.Command
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
