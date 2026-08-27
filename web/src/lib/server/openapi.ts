import catalog from '../../../../pkg/capabilityprotocol/generated/capability-tools.json';
import type { Locale } from '$lib/i18n/locale';
import { publicAPIPermissions } from '$lib/public-api-permission';

export type ApiDocumentationLanguage = Locale;

type CatalogTool = {
	name: string;
	namespace: string;
	description?: string;
	version: string;
	sideEffectClass?: string;
	requiresApproval?: boolean;
	requiresUserPresence: boolean;
	inputSchema?: Record<string, unknown>;
	outputSchema?: Record<string, unknown>;
};

type EndpointCopy = { summary: string; description: string };

type ApiCopy = {
	title: string;
	description: string;
	serverDescription: string;
	tags: Record<'token' | 'agent' | 'tools' | 'capabilities', string>;
	endpoints: Record<
		| 'createToken'
		| 'sendMessage'
		| 'readReplies'
		| 'listTools'
		| 'readTool'
		| 'invokeTool',
		EndpointCopy
	>;
	errors: Record<'badRequest' | 'unauthorized' | 'forbidden' | 'notFound' | 'badGateway', string>;
};

const localizedCopy: Record<ApiDocumentationLanguage, ApiCopy> = {
	ko: {
		title: '김인턴 API',
		description: [
			'김인턴이 사내에서 쓰는 도구를 그대로 외부에서 호출하는 API입니다. 토큰은 사람에게 발급되고,',
			'호출은 언제나 그 사람의 신원으로 실행됩니다. 토큰이 할 수 있는 일은 그 사람이 할 수 있는 일을',
			'넘지 않습니다.',
			'',
			'`POST /tokens`으로 토큰을 만들고, 이후 모든 요청에 `Authorization: Bearer <token>`을 담습니다.',
			'토큰 발급만은 관리 화면에 로그인한 세션으로 합니다.'
		].join('\n'),
		serverDescription: '모든 회사가 이 한 주소를 씁니다. 어느 회사인지는 토큰이 말합니다.',
		tags: {
			token: '토큰',
			agent: '에이전트에게 맡기기',
			tools: '도구 목록',
			capabilities: '도구'
		},
		endpoints: {
			createToken: {
				summary: 'API 토큰 발급',
				description:
					'로그인한 직원 본인 앞으로 토큰을 발급합니다. 응답의 `token`은 이때 한 번만 보여집니다.'
			},
			sendMessage: {
				summary: '에이전트에게 메시지 보내기',
				description:
					'메신저에 쓰는 것과 같은 방식으로 에이전트에게 일을 맡깁니다. `conversationID`를 생략하면 새 대화가 열리고, 응답이 그 ID를 알려줍니다.'
			},
			readReplies: {
				summary: '에이전트의 답 읽기',
				description: '한 대화에서 에이전트가 지금까지 보낸 답을 읽습니다.'
			},
			listTools: {
				summary: '이 토큰이 부를 수 있는 도구',
				description:
					'토큰의 scope로 부를 수 있는 도구만 돌려줍니다. 컴패니언 도구처럼 상황에 따라 생기고 사라지는 도구는 여기에서 확인합니다.'
			},
			readTool: { summary: '도구 하나의 명세', description: '입력 스키마와 부작용 등급을 포함합니다.' },
			invokeTool: {
				summary: '도구 부르기',
				description:
					'`input`은 그 도구의 입력 스키마를 따릅니다. `toolName`, `actor`, `context`는 토큰과 도구 명세에서 정해지므로 본문에 담아도 무시됩니다.'
			}
		},
		errors: {
			badRequest: '요청 본문이나 조건이 올바르지 않습니다',
			unauthorized: '토큰이 없거나 유효하지 않습니다',
			forbidden: '토큰의 scope로는 부를 수 없는 도구입니다',
			notFound: '그런 도구가 없습니다',
			badGateway: '기기 안쪽 서비스가 응답하지 않았습니다'
		}
	},
	en: {
		title: 'internkim API',
		description: [
			'The tools internkim uses inside a company, callable from outside it. A token belongs to a',
			'person, and every call runs as that person, so a token can never do more than its owner can.',
			'',
			'Create a token with `POST /tokens`, then send `Authorization: Bearer <token>` on every request.',
			'Creating the token itself uses a signed-in staff session.'
		].join('\n'),
		serverDescription: 'Every company calls this one address; the token says which company it is.',
		tags: {
			token: 'Tokens',
			agent: 'Asking the agent',
			tools: 'Discovery',
			capabilities: 'Tools'
		},
		endpoints: {
			createToken: {
				summary: 'Create an API token',
				description:
					'Issues a token to the signed-in member. The `token` field is shown this once and never again.'
			},
			sendMessage: {
				summary: 'Send the agent a message',
				description:
					'Asks the agent for work the way a message in the messenger does. Omit `conversationID` to open a new conversation; the response carries the one it opened.'
			},
			readReplies: {
				summary: "Read the agent's replies",
				description: 'Returns what the agent has said so far in one conversation.'
			},
			listTools: {
				summary: 'List the tools this token may call',
				description:
					"Returns only the tools the token's scopes reach. Tools that come and go with circumstance, such as the companion's, are discovered here."
			},
			readTool: {
				summary: 'Read one tool',
				description: 'Includes the input schema and the side-effect class.'
			},
			invokeTool: {
				summary: 'Invoke a tool',
				description:
					'`input` follows that tool\'s own input schema. `toolName`, `actor` and `context` come from the token and the descriptor, so a body that carries them is ignored.'
			}
		},
		errors: {
			badRequest: 'The body or the query is not usable',
			unauthorized: 'No token, or a token that is not valid',
			forbidden: "The token's scopes do not reach this tool",
			notFound: 'No tool by that name',
			badGateway: 'A service inside the device did not answer'
		}
	}
};

const defaultZone = 'intern.kim';

export function apiBaseURL(zone = defaultZone): string {
	return `https://api.${zone}/v1`;
}

export function baseTools(): CatalogTool[] {
	return [...(catalog.tools as CatalogTool[])].sort((left, right) =>
		left.name < right.name ? -1 : 1
	);
}

export function protocolVersion(): string {
	return catalog.protocolVersion;
}

function withoutSchemaDialect(schema: Record<string, unknown> | undefined): Record<string, unknown> {
	if (!schema) return { type: 'object' };
	const { $schema: _dialect, ...rest } = schema;
	return rest;
}

function errorResponse(description: string) {
	return { description, content: { 'text/plain': { schema: { type: 'string' } } } };
}

function jsonResponse(description: string, schemaName: string) {
	return {
		description,
		content: { 'application/json': { schema: { $ref: `#/components/schemas/${schemaName}` } } }
	};
}

function jsonBody(schemaName: string) {
	return {
		required: true,
		content: { 'application/json': { schema: { $ref: `#/components/schemas/${schemaName}` } } }
	};
}

function createTokenPath(copy: ApiCopy) {
	return {
		post: {
			tags: [copy.tags.token],
			operationId: 'createToken',
			summary: copy.endpoints.createToken.summary,
			description: copy.endpoints.createToken.description,
			security: [],
			requestBody: jsonBody('TokenRequest'),
			responses: {
				'200': jsonResponse(copy.endpoints.createToken.summary, 'TokenResponse'),
				'400': errorResponse(copy.errors.badRequest),
				'403': errorResponse(copy.errors.forbidden)
			}
		}
	};
}

function agentMessagePath(copy: ApiCopy) {
	return {
		post: {
			tags: [copy.tags.agent],
			operationId: 'sendAgentMessage',
			summary: copy.endpoints.sendMessage.summary,
			description: copy.endpoints.sendMessage.description,
			requestBody: jsonBody('AgentMessageRequest'),
			responses: {
				'200': jsonResponse(copy.endpoints.sendMessage.summary, 'AgentMessageResponse'),
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.forbidden),
				'502': errorResponse(copy.errors.badGateway)
			}
		}
	};
}

function agentRepliesPath(copy: ApiCopy) {
	return {
		get: {
			tags: [copy.tags.agent],
			operationId: 'readAgentReplies',
			summary: copy.endpoints.readReplies.summary,
			description: copy.endpoints.readReplies.description,
			parameters: [
				{
					name: 'conversationID',
					in: 'query',
					required: true,
					schema: { type: 'string' }
				}
			],
			responses: {
				'200': { description: copy.endpoints.readReplies.summary },
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.forbidden),
				'502': errorResponse(copy.errors.badGateway)
			}
		}
	};
}

function listToolsPath(copy: ApiCopy) {
	return {
		get: {
			tags: [copy.tags.tools],
			operationId: 'listTools',
			summary: copy.endpoints.listTools.summary,
			description: copy.endpoints.listTools.description,
			responses: {
				'200': jsonResponse(copy.endpoints.listTools.summary, 'ToolList'),
				'401': errorResponse(copy.errors.unauthorized),
				'502': errorResponse(copy.errors.badGateway)
			}
		}
	};
}

function toolByNamePath(copy: ApiCopy) {
	const nameParameter = {
		name: 'name',
		in: 'path',
		required: true,
		schema: { type: 'string' }
	};
	return {
		get: {
			tags: [copy.tags.tools],
			operationId: 'readTool',
			summary: copy.endpoints.readTool.summary,
			description: copy.endpoints.readTool.description,
			parameters: [nameParameter],
			responses: {
				'200': jsonResponse(copy.endpoints.readTool.summary, 'ToolDescriptor'),
				'401': errorResponse(copy.errors.unauthorized),
				'404': errorResponse(copy.errors.notFound)
			}
		}
	};
}

function invokeToolPath(copy: ApiCopy) {
	return {
		post: {
			tags: [copy.tags.tools],
			operationId: 'invokeTool',
			summary: copy.endpoints.invokeTool.summary,
			description: copy.endpoints.invokeTool.description,
			parameters: [{ name: 'name', in: 'path', required: true, schema: { type: 'string' } }],
			requestBody: jsonBody('ToolInvokeRequest'),
			responses: {
				'200': jsonResponse(copy.endpoints.invokeTool.summary, 'ToolInvokeResponse'),
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.forbidden),
				'502': errorResponse(copy.errors.badGateway)
			}
		}
	};
}

function describeTool(tool: CatalogTool): string {
	const facts = [
		`\`${tool.sideEffectClass ?? 'read'}\``,
		tool.requiresApproval ? 'approval' : '',
		tool.requiresUserPresence ? 'presence' : ''
	].filter(Boolean);
	return [tool.description ?? '', '', `${facts.join(' · ')} · v${tool.version}`].join('\n').trim();
}

function namedToolPath(tool: CatalogTool, copy: ApiCopy) {
	return {
		post: {
			tags: [`${copy.tags.capabilities}: ${tool.namespace}`],
			operationId: tool.name,
			summary: tool.name,
			description: describeTool(tool),
			requestBody: {
				required: true,
				content: {
					'application/json': {
						schema: {
							type: 'object',
							required: ['input'],
							properties: {
								input: withoutSchemaDialect(tool.inputSchema),
								idempotencyKey: { type: 'string' },
								timeoutSecond: { type: 'integer' }
							}
						}
					}
				}
			},
			responses: {
				'200': jsonResponse(tool.name, 'ToolInvokeResponse'),
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.forbidden),
				'502': errorResponse(copy.errors.badGateway)
			}
		}
	};
}

function createPaths(copy: ApiCopy) {
	const paths: Record<string, unknown> = {
		'/tokens': createTokenPath(copy),
		'/agent/messages': agentMessagePath(copy),
		'/agent/replies': agentRepliesPath(copy),
		'/tools': listToolsPath(copy),
		'/tools/{name}': toolByNamePath(copy),
		'/tools/{name}/invoke': invokeToolPath(copy)
	};
	for (const tool of baseTools()) {
		paths[`/tools/${tool.name}/invoke`] = namedToolPath(tool, copy);
	}
	return paths;
}

function createComponents(copy: ApiCopy) {
	return {
		securitySchemes: {
			memberToken: { type: 'http', scheme: 'bearer', description: copy.tags.token }
		},
		schemas: {
			TokenRequest: {
				type: 'object',
				properties: {
					label: { type: 'string' },
					scopes: { type: 'array', items: { type: 'string', enum: [...publicAPIPermissions] } }
				}
			},
			TokenResponse: {
				type: 'object',
				properties: {
					token: { type: 'string' },
					actor: { $ref: '#/components/schemas/ActorContext' },
					record: {
						type: 'object',
						properties: {
							id: { type: 'string' },
							label: { type: 'string' },
							email: { type: 'string', format: 'email' },
							scopes: { type: 'array', items: { type: 'string' } },
							createdAt: { type: 'string', format: 'date-time' }
						}
					}
				}
			},
			ActorContext: {
				type: 'object',
				properties: {
					personID: { type: 'string' },
					email: { type: 'string', format: 'email' },
					displayName: { type: 'string' },
					source: { type: 'string' },
					scopes: { type: 'array', items: { type: 'string' } },
					isAdmin: { type: 'boolean' }
				}
			},
			AgentMessageRequest: {
				type: 'object',
				required: ['message'],
				properties: {
					message: { type: 'string' },
					conversationID: { type: 'string' },
					messageID: { type: 'string' }
				}
			},
			AgentMessageResponse: {
				type: 'object',
				properties: {
					conversationID: { type: 'string' },
					messageID: { type: 'string' },
					result: {}
				}
			},
			ToolList: {
				type: 'object',
				properties: {
					tools: { type: 'array', items: { $ref: '#/components/schemas/ToolDescriptor' } }
				}
			},
			ToolDescriptor: {
				type: 'object',
				properties: {
					name: { type: 'string' },
					namespace: { type: 'string' },
					description: { type: 'string' },
					version: { type: 'string' },
					sideEffectClass: { type: 'string' },
					requiresApproval: { type: 'boolean' },
					requiresUserPresence: { type: 'boolean' },
					inputSchema: { type: 'object' },
					outputSchema: { type: 'object' }
				}
			},
			ToolInvokeRequest: {
				type: 'object',
				required: ['input'],
				properties: {
					input: { type: 'object' },
					idempotencyKey: { type: 'string' },
					timeoutSecond: { type: 'integer' }
				}
			},
			ToolInvokeResponse: {
				type: 'object',
				properties: {
					toolName: { type: 'string' },
					status: { type: 'string' },
					content: { type: 'string' },
					isError: { type: 'boolean' },
					message: { type: 'string' },
					errorCode: { type: 'string' },
					retryable: { type: 'boolean' },
					result: {}
				}
			}
		}
	};
}

export function createOpenApiDocument(language: ApiDocumentationLanguage, zone = defaultZone) {
	const copy = localizedCopy[language];
	return {
		openapi: '3.1.0',
		info: {
			title: copy.title,
			version: protocolVersion(),
			description: copy.description
		},
		servers: [{ url: apiBaseURL(zone), description: copy.serverDescription }],
		security: [{ memberToken: [] }],
		paths: createPaths(copy),
		components: createComponents(copy)
	};
}
