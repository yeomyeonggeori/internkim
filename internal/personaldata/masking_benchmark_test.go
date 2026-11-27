package personaldata

import (
	"encoding/json"
	"strings"
	"testing"
)

func conversationOf(turns int) []byte {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]message, 0, turns)
	for turn := range turns {
		text := strings.Repeat("회의는 화요일 오후 2시이고 3분기 매출은 12억입니다. ", 20)
		if turn%10 == 0 {
			text += "제 번호는 010-1234-5678이에요."
		}
		messages = append(messages, message{Role: "user", Content: text})
	}
	document, _ := json.Marshal(map[string]any{"model": "example/model", "messages": messages})
	return document
}

func benchmarkMasking(b *testing.B, turns int) {
	document := conversationOf(turns)
	masker := NewMasker()
	b.SetBytes(int64(len(document)))
	b.ResetTimer()
	for range b.N {
		masker.MaskJSON(document, NewSubstitutions())
	}
}

func BenchmarkMaskingAConversationOf8KTokens(b *testing.B)   { benchmarkMasking(b, 27) }
func BenchmarkMaskingAConversationOf32KTokens(b *testing.B)  { benchmarkMasking(b, 107) }
func BenchmarkMaskingAConversationOf128KTokens(b *testing.B) { benchmarkMasking(b, 427) }
