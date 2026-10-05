import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { WebStandardStreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/webStandardStreamableHttp.js';
import { CallToolRequestSchema, ListToolsRequestSchema } from '@modelcontextprotocol/sdk/types.js';
import type { CallToolResult, Tool, ToolAnnotations } from '@modelcontextprotocol/sdk/types.js';
import { capabilityAnsweredFilesMetaKey, capabilityDescriptorMetaKey } from './catalog/protocol';
import { isSeenByAModel, permissionForTool, toolsAModelReachesWith } from '$lib/server/public-api/catalog';
import type { ToolDescriptor } from '$lib/server/public-api/catalog';
import { toolAnswerOrRefusal } from '$lib/server/public-api/tool-call';
import type { CallingMember } from '$lib/server/member-request';
import type { Environment } from '$lib/server/agent-request';
import { filesAnsweredBy, type AnsweredFile } from '$lib/server/public-api/record/answered-files';

export { capabilityDescriptorMetaKey as descriptorMetaKey } from './catalog/protocol';

const serverInstructions =
	'These are the tools of one company on internkim, answered as the person whose token asked. ' +
	'A tool call carries the same input the public API takes and answers the same body.';

export function toolsOfferedTo(member: CallingMember): Tool[] {
	return toolsAModelReachesWith(member.permission).map(mcpToolOf);
}

export function annotationsOf(descriptor: ToolDescriptor): ToolAnnotations {
	return {
		...(permissionForTool(descriptor) === 'read' ? { readOnlyHint: true } : {}),
		...(descriptor.requiresApproval ? { destructiveHint: true } : {})
	};
}

function mcpToolOf(descriptor: ToolDescriptor): Tool {
	return {
		name: descriptor.name,
		description: descriptor.description,
		inputSchema: descriptor.inputSchema as Tool['inputSchema'],
		annotations: annotationsOf(descriptor),
		_meta: { [capabilityDescriptorMetaKey]: descriptor }
	};
}

function toolResultOf(status: number, body: unknown, files: AnsweredFile[] = []): CallToolResult {
	return {
		content: [{ type: 'text', text: JSON.stringify(body) }, ...files.map(embeddedResourceOf)],
		structuredContent: body as CallToolResult['structuredContent'],
		isError: status >= 300
	};
}

function embeddedResourceOf(file: AnsweredFile): CallToolResult['content'][number] {
	const uri = `internkim://files/${encodeURIComponent(file.name)}`;
	if (file.bytes) {
		return { type: 'resource', resource: { uri, mimeType: file.mimeType, blob: base64Of(file.bytes) } };
	}
	return { type: 'resource', resource: { uri, mimeType: file.mimeType, text: file.text ?? '' } };
}

function base64Of(bytes: Uint8Array): string {
	let written = '';
	for (let offset = 0; offset < bytes.length; offset += 0x8000) {
		written += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
	}
	return btoa(written);
}

function keepsAnsweredFiles(meta: Record<string, unknown> | undefined): boolean {
	return meta?.[capabilityAnsweredFilesMetaKey] === 'kept';
}

function companyToolServer(environment: Environment, member: CallingMember): Server {
	const server = new Server(
		{ name: 'internkim', version: '1' },
		{ capabilities: { tools: {} }, instructions: serverInstructions }
	);

	server.setRequestHandler(ListToolsRequestSchema, () => ({ tools: toolsOfferedTo(member) }));

	server.setRequestHandler(CallToolRequestSchema, async (request) => {
		if (!isSeenByAModel(request.params.name)) {
			return toolResultOf(404, { message: `no tool here goes by ${request.params.name}` });
		}
		const answered = await toolAnswerOrRefusal(environment, member, request.params.name, {
			input: request.params.arguments ?? {}
		});
		if (answered.status >= 300 || !keepsAnsweredFiles(request.params._meta)) {
			return toolResultOf(answered.status, answered.body);
		}
		return toolResultOf(answered.status, answered.body, await filesAnsweredBy(request.params.name, member, answered.body));
	});

	return server;
}

export async function answerMCP(
	request: Request,
	environment: Environment,
	member: CallingMember
): Promise<Response> {
	const transport = new WebStandardStreamableHTTPServerTransport({
		sessionIdGenerator: undefined,
		enableJsonResponse: true
	});
	const server = companyToolServer(environment, member);
	await server.connect(transport);
	try {
		return await transport.handleRequest(request);
	} finally {
		await server.close();
	}
}
