package cli

import (
	"strings"
	"testing"
)

func TestVerifyAPIScriptChecksLiteRTWithCPUAccelerator(t *testing.T) {
	script := verifyAPIScript()
	requiredFragments := []string{
		`accelerator: "cpu"`,
		`python3 -c 'import json, sys; document=json.load(sys.stdin); backend=document.get("selectedBackend"); content=document.get("content"); raise SystemExit(0 if backend in ("gpu", "cpu") and isinstance(content, str) and len(content) > 0 else 1)'`,
		`litert capability: %s`,
		`litert capability: optional local check failed`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected API verify script to include %q", fragment)
		}
	}
	if strings.Contains(script, `jq -e '.selectedBackend as $backend`) {
		t.Fatal("expected LiteRT verify to avoid raw jq parsing of possibly non-JSON responses")
	}
}

func TestVerifyAPIScriptChecksRemoteStructuredLLM(t *testing.T) {
	script := verifyAPIScript()
	requiredFragments := []string{
		`executionMode: "remote"`,
		`executionMode: "auto"`,
		`http://internkim/v1/llm/structured`,
		`--argjson schema "$schema"`,
		`structuredOutputSchema: {name:"smoke_reply", document:$schema, isStrictlyEnforced:true}`,
		`structuredOutputSchema: {name:"bluecollar_agent_turn_action", document:$schema, isStrictlyEnforced:true}`,
		`.provider == "openrouter" and .selectedBackend == "remote"`,
		`.constraintMode == "native_tool_call"`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected API verify script to include %q", fragment)
		}
	}
}
