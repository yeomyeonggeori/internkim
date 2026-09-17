import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { WebStandardStreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/webStandardStreamableHttp.js';
import { CallToolRequestSchema, ListToolsRequestSchema } from '@modelcontextprotocol/sdk/types.js';
import type { CallToolResult, Tool } from '@modelcontextprotocol/sdk/types.js';
import { capabilityDescriptorMetaKey } from './catalog/protocol';
import { isSeenByAModel, toolsAModelReachesWith } from '$lib/server/public-api/catalog';
import type { ToolDescriptor } from '$lib/server/public-api/catalog';
import { toolAnswerOrRefusal } from '$lib/server/public-api/tool-call';
import type { CallingMember } from '$lib/server/member-request';
import type { Environment } from '$lib/server/agent-request';

export { capabilityDescriptorMetaKey as descriptorMetaKey } from './catalog/protocol';

const serverInstructions =
	'These are the tools of one company on internkim, answered as the person whose token asked. ' +
	'A tool call carries the same input the public API takes and answers the same body.';

export function toolsOfferedTo(member: CallingMember): Tool[] {
	return toolsAModelReachesWith(member.permission).map(mcpToolOf);
}

function mcpToolOf(descriptor: ToolDescriptor): Tool {
	return {
		name: descriptor.name,
		description: descriptor.description,
		inputSchema: descriptor.inputSchema as Tool['inputSchema'],
		_meta: { [capabilityDescriptorMetaKey]: descriptor }
	};
}

function toolResultOf(status: number, body: unknown): CallToolResult {
	return {
		content: [{ type: 'text', text: JSON.stringify(body) }],
		structuredContent: body as CallToolResult['structuredContent'],
		isError: status >= 300
	};
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
		return toolResultOf(answered.status, answered.body);
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
