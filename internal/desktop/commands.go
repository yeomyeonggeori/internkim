package desktop

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sort"
)

//go:embed commands.json
var commandsDocument []byte

const DefaultProgram = "agent-desktop"

const (
	textByStandardInput = "standardInput"
	textByLastArgument  = "lastArgument"
)

type Program struct {
	TextTravelsBy     string   `json:"textTravelsBy"`
	AppendedArguments []string `json:"appendedArguments"`
	Read              []string `json:"read"`
	Act               []string `json:"act"`
}

var Programs = mustReadPrograms()

func mustReadPrograms() map[string]Program {
	var document struct {
		Programs map[string]Program `json:"programs"`
	}
	if errorValue := json.Unmarshal(commandsDocument, &document); errorValue != nil {
		panic("internal/desktop/commands.json: " + errorValue.Error())
	}
	return document.Programs
}

func ProgramNames() []string {
	names := make([]string, 0, len(Programs))
	for name := range Programs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (program Program) Reads(command string) bool {
	return slices.Contains(program.Read, command)
}

func (program Program) Acts(command string) bool {
	return slices.Contains(program.Act, command)
}
