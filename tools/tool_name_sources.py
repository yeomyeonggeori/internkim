"""The places a tool name is declared, read once for every check that needs them.

Three repositories each declare tool names in their own file, and no build step
reconciles them: the generated capability catalog here, bluecollar's kernel
tools, and blueclaw's local tool provider. A check that reads only one of them
sees a name and cannot tell whether anything answers to it.
"""

import json
import pathlib
import re

REPOSITORY_ROOT = pathlib.Path(__file__).resolve().parent.parent

GENERATED_CATALOG = (
    REPOSITORY_ROOT / "pkg" / "capabilityprotocol" / "generated" / "capability-tools.json"
)
CAPABILITY_DESCRIPTORS = REPOSITORY_ROOT / "internal" / "capabilities" / "protocol.go"
BLUECLAW_ROOT = REPOSITORY_ROOT / ".dependency" / "blueclaw"
BLUECLAW_LOCAL_TOOLS = BLUECLAW_ROOT / "internal" / "agentruntime" / "local_tool_provider.go"
BLUECOLLAR_ROOT = BLUECLAW_ROOT / ".dependency" / "bluecollar"
KERNEL_TOOLS = BLUECOLLAR_ROOT / "toolcontract" / "kernel_tools.go"


def names_in(path: pathlib.Path, pattern: str) -> set[str]:
    if not path.is_file():
        return set()
    return set(re.findall(pattern, path.read_text()))


def catalog_tools() -> list[dict]:
    return json.loads(GENERATED_CATALOG.read_text())["tools"]


def model_name_of(tool: dict) -> str:
    return tool.get("modelName") or tool["name"]


def model_visible_catalog_names() -> set[str]:
    return {model_name_of(tool) for tool in catalog_tools() if tool.get("modelVisibility") != "hidden"}


def model_hidden_tool_names() -> set[str]:
    return {tool["name"] for tool in catalog_tools() if tool.get("modelVisibility") == "hidden"}


def capability_descriptor_names() -> set[str]:
    return names_in(CAPABILITY_DESCRIPTORS, r'\{Name: "([a-z0-9_]+)"')


def blueclaw_local_tool_names() -> set[str]:
    return names_in(BLUECLAW_LOCAL_TOOLS, r'Name:\s+"([a-z0-9_]+)"')


def kernel_tool_names() -> set[str]:
    return names_in(KERNEL_TOOLS, r'ToolName\s+= "([a-z0-9_.]+)"')


def served_tool_names() -> set[str]:
    names = {tool["name"] for tool in catalog_tools()}
    names |= capability_descriptor_names()
    names |= blueclaw_local_tool_names()
    names |= kernel_tool_names()
    return names
