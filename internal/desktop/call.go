package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type Kind string

const (
	KindRead Kind = "read"
	KindAct  Kind = "act"
)

type Call struct {
	Program   string   `json:"program"`
	Arguments []string `json:"arguments"`
	Text      string   `json:"text,omitempty"`
}

type Answer struct {
	Output  string `json:"output,omitempty"`
	Usage   string `json:"usage,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

func (call Call) UsageHeading() string {
	return call.Program + " " + call.Arguments[0] + " --help"
}

const textFromStandardInput = "--stdin"

func NewCall(kind Kind, programName string, arguments []string, text string) (Call, error) {
	if programName == "" {
		programName = DefaultProgram
	}
	program, isKnown := Programs[programName]
	if !isKnown {
		return Call{}, fmt.Errorf("%q is not a program this tool runs; program is one of: %s", programName, strings.Join(ProgramNames(), ", "))
	}
	if len(arguments) == 0 {
		return Call{}, errors.New("arguments is empty; the first argument is the command, for example [\"snapshot\", \"--app\", \"Finder\", \"-i\"]")
	}
	command := arguments[0]
	if !program.isOfKind(kind, command) {
		return Call{}, errors.New(program.wrongCommandMessage(programName, kind, command))
	}
	if refused, isRefused := program.refusedIn(arguments); isRefused {
		return Call{}, fmt.Errorf("%s is never run with %q here", programName, refused)
	}
	call := Call{Program: programName, Arguments: slices.Concat(program.Observation.sized(arguments), program.AppendedArguments)}
	if text == "" {
		return call, nil
	}
	if program.TextTravelsBy == textByLastArgument {
		call.Arguments = append(call.Arguments, text)
		return call, nil
	}
	call.Arguments = append(call.Arguments, textFromStandardInput)
	call.Text = text
	return call, nil
}

func (program Program) isOfKind(kind Kind, command string) bool {
	return slices.Contains(program.commandsOf(kind), command)
}

func (program Program) wrongCommandMessage(programName string, kind Kind, command string) string {
	if kind == KindRead && program.Acts(command) {
		return fmt.Sprintf("%s %q changes the desktop; call desktop_act with the same program and arguments", programName, command)
	}
	return fmt.Sprintf("%q is not a %s command this tool runs; the first argument is one of: %s", command, programName, strings.Join(program.commandsOf(kind), ", "))
}

func (program Program) commandsOf(kind Kind) []string {
	if kind == KindRead {
		return program.Read
	}
	return slices.Concat(program.Act, program.Read)
}

type Envelope struct {
	OK      bool         `json:"ok"`
	Success bool         `json:"success"`
	Data    EnvelopeData `json:"data"`
}

type EnvelopeData struct {
	SnapshotID  string `json:"snapshot_id"`
	Application string `json:"app"`
	Window      struct {
		ID string `json:"id"`
	} `json:"window"`
	Disposition struct {
		Delivery string `json:"delivery"`
	} `json:"disposition"`
}

func (data *EnvelopeData) UnmarshalJSON(document []byte) error {
	type fields EnvelopeData
	var read fields
	if json.Unmarshal(document, &read) == nil {
		*data = EnvelopeData(read)
	}
	return nil
}

func (envelope Envelope) Succeeded() bool {
	return envelope.OK || envelope.Success
}

const deliveryOfNothing = "not_delivered"

func (envelope Envelope) Delivered() bool {
	return envelope.Succeeded() && envelope.Data.Disposition.Delivery != deliveryOfNothing
}

type Screenshot struct {
	PNGBase64 string
	Remainder string
}

func ReadEnvelope(output string) (Envelope, error) {
	var envelope Envelope
	if errorValue := json.Unmarshal([]byte(lastLineOf(output)), &envelope); errorValue != nil {
		return Envelope{}, errors.New("the program answered something other than its json envelope: " + truncated(output))
	}
	return envelope, nil
}

func ScreenshotOf(output string) (Screenshot, bool) {
	var envelope map[string]json.RawMessage
	if json.Unmarshal([]byte(lastLineOf(output)), &envelope) != nil {
		return Screenshot{}, false
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(envelope["data"], &data) != nil {
		return Screenshot{}, false
	}
	var picture, format string
	if json.Unmarshal(data["data"], &picture) != nil || json.Unmarshal(data["format"], &format) != nil {
		return Screenshot{}, false
	}
	if picture == "" || format != "png" {
		return Screenshot{}, false
	}
	delete(data, "data")
	envelope["data"], _ = json.Marshal(data)
	remainder, _ := json.Marshal(envelope)
	return Screenshot{PNGBase64: picture, Remainder: string(remainder)}, true
}

func lastLineOf(output string) string {
	trimmed := strings.TrimSpace(output)
	if index := strings.LastIndexByte(trimmed, '\n'); index >= 0 {
		return strings.TrimSpace(trimmed[index+1:])
	}
	return trimmed
}

const longestQuotedOutput = 400

func truncated(output string) string {
	trimmed := strings.TrimSpace(output)
	if len(trimmed) <= longestQuotedOutput {
		return trimmed
	}
	return trimmed[:longestQuotedOutput] + "…"
}
