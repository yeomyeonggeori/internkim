package personaldata

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var tokenPrefixPattern = regexp.MustCompile(`^\{(?:\{(?:[A-Za-z]+(?::[0-9a-f]{0,64}\}?)?)?)?$`)

type pathSegment struct {
	key       string
	element   int
	isElement bool
}

type heldText struct {
	path []pathSegment
	text string
}

type unmaskingEventStream struct {
	source        *bufio.Reader
	body          io.Closer
	substitutions *Substitutions
	held          map[string]heldText
	lastScalars   map[string]any
	ready         bytes.Buffer
	endError      error
}

func (substitutions *Substitutions) UnmaskingEventStream(body io.ReadCloser) io.ReadCloser {
	return &unmaskingEventStream{source: bufio.NewReader(body), body: body, substitutions: substitutions, held: map[string]heldText{}, lastScalars: map[string]any{}}
}

func (stream *unmaskingEventStream) Read(buffer []byte) (int, error) {
	for stream.ready.Len() == 0 {
		if stream.endError != nil {
			return 0, stream.endError
		}
		line, errorValue := stream.source.ReadBytes('\n')
		stream.ready.Write(stream.translate(line))
		if errorValue != nil {
			stream.ready.Write(stream.eventForHeldText())
			stream.endError = errorValue
		}
	}
	return stream.ready.Read(buffer)
}

func (stream *unmaskingEventStream) Close() error {
	log.Printf("model_route.restored_personal_data: %s", stream.substitutions.RestoredSummary())
	return stream.body.Close()
}

func (stream *unmaskingEventStream) translate(line []byte) []byte {
	content := bytes.TrimRight(line, "\r\n")
	payload, isData := bytes.CutPrefix(content, []byte("data:"))
	if !isData {
		return line
	}
	payload = bytes.TrimSpace(payload)
	if string(payload) == "[DONE]" {
		return append(stream.eventForHeldText(), line...)
	}
	event, isObject := decodeObject(payload)
	if !isObject {
		return line
	}
	return append(append([]byte("data: "), stream.unmaskEvent(event)...), line[len(content):]...)
}

func (stream *unmaskingEventStream) unmaskEvent(event map[string]any) []byte {
	isFinishing := false
	walkStrings(event, nil, func(path []pathSegment, value string) string {
		if isFinishReason(path) && value != "" {
			isFinishing = true
		}
		return stream.unmaskFragment(path, value)
	})
	stream.rememberScalars(event)
	if isFinishing {
		stream.appendHeldTextTo(event)
	}
	return encodeEvent(event)
}

func (stream *unmaskingEventStream) unmaskFragment(path []pathSegment, value string) string {
	key := pathKey(path)
	ready, rest := splitBeforeTokenPrefix(stream.held[key].text + value)
	if rest == "" {
		delete(stream.held, key)
	} else {
		stream.held[key] = heldText{path: path, text: rest}
	}
	return stream.substitutions.UnmaskText(ready)
}

func (stream *unmaskingEventStream) eventForHeldText() []byte {
	if len(stream.held) == 0 {
		return nil
	}
	event := map[string]any{}
	for key, value := range stream.lastScalars {
		event[key] = value
	}
	stream.appendHeldTextTo(event)
	return append(append([]byte("data: "), encodeEvent(event)...), "\n\n"...)
}

func (stream *unmaskingEventStream) appendHeldTextTo(event map[string]any) {
	for key, held := range stream.held {
		appendAtPath(event, held.path, held.text)
		delete(stream.held, key)
	}
}

func (stream *unmaskingEventStream) rememberScalars(event map[string]any) {
	for key, value := range event {
		switch value.(type) {
		case map[string]any, []any:
		default:
			stream.lastScalars[key] = value
		}
	}
}

func splitBeforeTokenPrefix(text string) (string, string) {
	start := strings.LastIndex(text, "{")
	if start < 0 {
		return text, ""
	}
	if start > 0 && text[start-1] == '{' {
		start--
	}
	if !tokenPrefixPattern.MatchString(text[start:]) {
		return text, ""
	}
	return text[:start], text[start:]
}

func walkStrings(value any, path []pathSegment, visit func([]pathSegment, string) string) any {
	switch typed := value.(type) {
	case string:
		return visit(path, typed)
	case map[string]any:
		for key, child := range typed {
			typed[key] = walkStrings(child, extended(path, pathSegment{key: key}), visit)
		}
	case []any:
		for position, child := range typed {
			typed[position] = walkStrings(child, extended(path, elementSegment(child, position)), visit)
		}
	}
	return value
}

func appendAtPath(value any, path []pathSegment, text string) any {
	if len(path) == 0 {
		existing, _ := value.(string)
		return existing + text
	}
	head := path[0]
	if head.isElement {
		elements, _ := value.([]any)
		position := elementPosition(elements, head.element)
		if position < 0 {
			elements = append(elements, map[string]any{"index": json.Number(strconv.Itoa(head.element))})
			position = len(elements) - 1
		}
		elements[position] = appendAtPath(elements[position], path[1:], text)
		return elements
	}
	object, isObject := value.(map[string]any)
	if !isObject {
		object = map[string]any{}
	}
	object[head.key] = appendAtPath(object[head.key], path[1:], text)
	return object
}

func elementSegment(element any, position int) pathSegment {
	if index, hasIndex := declaredIndex(element); hasIndex {
		return pathSegment{element: index, isElement: true}
	}
	return pathSegment{element: position, isElement: true}
}

func elementPosition(elements []any, element int) int {
	for position, candidate := range elements {
		if index, hasIndex := declaredIndex(candidate); hasIndex && index == element {
			return position
		}
	}
	if element < len(elements) {
		if _, hasIndex := declaredIndex(elements[element]); !hasIndex {
			return element
		}
	}
	return -1
}

func declaredIndex(element any) (int, bool) {
	object, isObject := element.(map[string]any)
	if !isObject {
		return 0, false
	}
	number, isNumber := object["index"].(json.Number)
	if !isNumber {
		return 0, false
	}
	index, errorValue := strconv.Atoi(number.String())
	return index, errorValue == nil
}

func isFinishReason(path []pathSegment) bool {
	return len(path) > 0 && path[len(path)-1].key == "finish_reason"
}

func extended(path []pathSegment, next pathSegment) []pathSegment {
	return append(append([]pathSegment{}, path...), next)
}

func pathKey(path []pathSegment) string {
	var key strings.Builder
	for _, segment := range path {
		if segment.isElement {
			key.WriteString("[" + strconv.Itoa(segment.element) + "]")
			continue
		}
		key.WriteString("." + strconv.Quote(segment.key))
	}
	return key.String()
}

func decodeObject(payload []byte) (map[string]any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var event map[string]any
	if decoder.Decode(&event) != nil || event == nil {
		return nil, false
	}
	return event, true
}

func encodeEvent(event map[string]any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.Encode(event)
	return bytes.TrimRight(buffer.Bytes(), "\n")
}
