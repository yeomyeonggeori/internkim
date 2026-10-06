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
BLUEPROTOCOL_ROOT = BLUECLAW_ROOT / ".dependency" / "blueprotocol"
KERNEL_TOOLS = BLUEPROTOCOL_ROOT / "toolcontract" / "kernel_tools.go"


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


def kernel_tool_name_by_constant() -> dict[str, str]:
    if not KERNEL_TOOLS.is_file():
        return {}
    return dict(re.findall(r'(\w+ToolName)\s+= "([a-z0-9_.]+)"', KERNEL_TOOLS.read_text()))


def blueclaw_local_tool_names() -> set[str]:
    if not BLUECLAW_LOCAL_TOOLS.is_file():
        return set()
    source = BLUECLAW_LOCAL_TOOLS.read_text()
    name_by_constant = kernel_tool_name_by_constant()
    referenced_constants = set(re.findall(r"Name:\s+toolcontract\.(\w+ToolName)", source))
    unresolved = sorted(referenced_constants - name_by_constant.keys())
    if unresolved:
        raise LookupError(
            f"{BLUECLAW_LOCAL_TOOLS.name} names tools through {', '.join(unresolved)},"
            f" which {KERNEL_TOOLS.name} does not declare"
        )
    literal_names = set(re.findall(r'Name:\s+"([a-z0-9_]+)"', source))
    return literal_names | {name_by_constant[constant] for constant in referenced_constants}


def kernel_tool_names() -> set[str]:
    return set(kernel_tool_name_by_constant().values())


def served_tool_names() -> set[str]:
    names = {tool["name"] for tool in catalog_tools()}
    names |= capability_descriptor_names()
    names |= blueclaw_local_tool_names()
    names |= kernel_tool_names()
    return names
