import catalog from '../../../../pkg/capabilityprotocol/generated/capability-tools.json';
import { publicAPIPermissions } from '../../../../web/src/lib/public-api-permission';

export type ApiDocumentationLanguage = 'ko' | 'en';

type CatalogTool = {
	name: string;
	namespace: string;
	description?: string;
	version: string;
	sideEffectClass?: string;
	requiresApproval?: boolean;
	requiresUserPresence: boolean;
	idempotency?: { supported?: boolean };
	inputSchema?: Record<string, unknown>;
	outputSchema?: Record<string, unknown>;
};

type EndpointCopy = { summary: string; description: string; idempotency?: string };

type ApiCopy = {
	title: string;
	description: string;
	serverDescription: string;
	tags: Record<'token' | 'agent' | 'tools' | 'capabilities', string>;
	endpoints: Record<
		| 'listTokens'
		| 'createToken'
		| 'revokeToken'
		| 'uploadFile'
		| 'sendMessage'
		| 'readReplies'
		| 'listTools'
		| 'readTool'
		| 'invokeTool',
		EndpointCopy
	>;
	errors: Record<
		| 'badRequest'
		| 'unauthorized'
		| 'forbidden'
		| 'notFound'
		| 'badGateway'
		| 'aboveOwnRung'
		| 'namesItself'
		| 'revokesItself'
		| 'noSuchToken'
		| 'wrongTokenPath'
		| 'tooLarge',
		string
	>;
};

const localizedCopy: Record<ApiDocumentationLanguage, ApiCopy> = {
	ko: {
		title: '김인턴 API',
		description: [
			'김인턴이 사내에서 쓰는 도구를 그대로 외부에서 호출하는 API입니다. 토큰은 사람에게 발급되고,',
			'호출은 언제나 그 사람의 신원으로 실행됩니다. 토큰이 할 수 있는 일은 그 사람이 할 수 있는 일을',
			'넘지 않습니다.',
			'',
			'첫 토큰은 설정 화면에서 로그인한 채로 만듭니다. 그 뒤로는 토큰이 자기 후임을 만들 수 있어서,',
			'`POST /token`으로 새 토큰을 받고 `DELETE /token`으로 옛 토큰을 폐기하면 브라우저 없이 교체됩니다.',
			'토큰은 자기보다 높은 등급을 만들 수 없고 자기 자신도 폐기할 수 없습니다.',
			'모든 요청에는 `Authorization: Bearer <token>`을 담습니다.'
		].join('\n'),
		serverDescription: '모든 회사가 이 한 주소를 씁니다. 어느 회사인지는 토큰이 말합니다.',
		tags: {
			token: '토큰',
			agent: '에이전트에게 맡기기',
			tools: '도구 목록',
			capabilities: '도구'
		},
		endpoints: {
			listTokens: {
				summary: '토큰 목록',
				description:
					'이 토큰의 주인이 가진 토큰의 이름과 등급을 나열합니다. 토큰 값은 어디에도 남아 있지 않아 답하지 않습니다.'
			},
			createToken: {
				summary: '토큰 발급',
				description:
					'부른 토큰과 같은 사람 앞으로 새 토큰을 발급합니다. `permission`을 생략하면 부른 토큰과 같은 등급이 되고, 그보다 높은 등급은 만들 수 없습니다. `name`을 생략하면 아직 안 쓰인 첫 번째 번호를 붙입니다. 응답의 `token`은 이때 한 번만 보여집니다.'
			},
			revokeToken: {
				summary: '토큰 폐기',
				description:
					'이름으로 토큰 하나를 폐기합니다. 인증에 쓴 그 토큰은 폐기할 수 없습니다. 새 토큰이 답하는 걸 확인한 뒤 그것으로 옛 토큰을 폐기하는 것이 교체 순서입니다.'
			},
			uploadFile: {
				summary: '파일 올리기',
				description:
					'첨부로 쓸 파일을 올리고, 도구에 넘길 수 있는 자리를 돌려받습니다.'
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
					'`input`은 그 도구의 입력 스키마를 따릅니다. `toolName`, `actor`, `context`는 토큰과 도구 명세에서 정해지므로 본문에 담아도 무시됩니다.',
				idempotency:
					'같은 키로 다시 부르면 이 도구는 같은 일을 두 번 하지 않습니다. 이 필드는 그것을 지원하는 도구에만 있습니다.'
			}
		},
		errors: {
			badRequest: '요청 본문이나 조건이 올바르지 않습니다',
			unauthorized: '토큰이 없거나 유효하지 않습니다',
			forbidden: '이 토큰의 등급으로는 부를 수 없는 도구입니다',
			notFound: '그런 도구가 없습니다',
			badGateway: '회사 안쪽 서비스가 응답하지 않았습니다',
			aboveOwnRung: '자기보다 높은 등급의 토큰은 만들 수 없습니다',
			namesItself: '그 이름은 이 호출을 인증한 토큰의 것입니다',
			revokesItself: '토큰은 자기 자신을 폐기할 수 없습니다. 자기를 대체한 토큰으로 폐기하세요',
			noSuchToken: '그런 이름의 토큰이 없습니다',
			wrongTokenPath: '토큰 하나를 만들고 폐기하는 곳은 /token 입니다',
			tooLarge: '파일이 너무 큽니다'
		}
	},
	en: {
		title: 'internkim API',
		description: [
			'The tools internkim uses inside a company, callable from outside it. A token belongs to a',
			'person, and every call runs as that person, so a token can never do more than its owner can.',
			'',
			'The first token is made signed in, from the settings screen. After that a token makes its own',
			'successors: `POST /token` for a new one, `DELETE /token` for the old one, so a script rotates',
			'without a browser. A token can neither mint a rung above its own nor revoke itself.',
			'Send `Authorization: Bearer <token>` on every request.'
		].join('\n'),
		serverDescription: 'Every company calls this one address; the token says which company it is.',
		tags: {
			token: 'Tokens',
			agent: 'Asking the agent',
			tools: 'Discovery',
			capabilities: 'Tools'
		},
		endpoints: {
			listTokens: {
				summary: 'List tokens',
				description:
					"The names and rungs of the tokens this token's owner holds. No token value is kept anywhere, so none is answered."
			},
			createToken: {
				summary: 'Make a token',
				description:
					'Issues a token to the same person as the one calling. `permission` defaults to the calling rung and may not reach past it. Omitting `name` takes the first ordinal nobody holds. The `token` field is shown this once and never again.'
			},
			revokeToken: {
				summary: 'Revoke a token',
				description:
					'Revokes one token by name. The token authenticating the call cannot be the one revoked: see the replacement answer first, then revoke the old one with it.'
			},
			uploadFile: {
				summary: 'Upload a file',
				description: 'Keeps a file for use as an attachment and answers with the place a tool can be handed.'
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
					'`input` follows that tool\'s own input schema. `toolName`, `actor` and `context` come from the token and the descriptor, so a body that carries them is ignored.',
				idempotency:
					'Calling again with the same key makes this tool do the same work once. Only tools that support it carry this field.'
			}
		},
		errors: {
			badRequest: 'The body or the query is not usable',
			unauthorized: 'No token, or a token that is not valid',
			forbidden: "This token's rung does not reach this tool",
			notFound: 'No tool by that name',
			badGateway: 'A service inside the company did not answer',
			aboveOwnRung: 'A token may not make one that reaches past itself',
			namesItself: 'That name belongs to the token making this call',
			revokesItself: 'A token cannot revoke itself; revoke it with the one that replaced it',
			noSuchToken: 'No token of yours goes by that name',
			wrongTokenPath: 'One token is made and revoked at /token',
			tooLarge: 'That file is too large'
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

function listTokensPath(copy: ApiCopy) {
	return {
		get: {
			tags: [copy.tags.token],
			operationId: 'listTokens',
			summary: copy.endpoints.listTokens.summary,
			description: copy.endpoints.listTokens.description,
			responses: {
				'200': jsonResponse(copy.endpoints.listTokens.summary, 'TokenList'),
				'401': errorResponse(copy.errors.unauthorized),
				'405': errorResponse(copy.errors.wrongTokenPath)
			}
		}
	};
}

function tokenPath(copy: ApiCopy) {
	return {
		post: {
			tags: [copy.tags.token],
			operationId: 'createToken',
			summary: copy.endpoints.createToken.summary,
			description: copy.endpoints.createToken.description,
			requestBody: jsonBody('TokenRequest'),
			responses: {
				'200': jsonResponse(copy.endpoints.createToken.summary, 'TokenResponse'),
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.aboveOwnRung),
				'409': errorResponse(copy.errors.namesItself)
			}
		},
		delete: {
			tags: [copy.tags.token],
			operationId: 'revokeToken',
			summary: copy.endpoints.revokeToken.summary,
			description: copy.endpoints.revokeToken.description,
			parameters: [
				{
					name: 'name',
					in: 'query',
					required: true,
					schema: { type: 'string' },
					description: copy.endpoints.revokeToken.summary
				}
			],
			responses: {
				'200': jsonResponse(copy.endpoints.revokeToken.summary, 'TokenRevoked'),
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'404': errorResponse(copy.errors.noSuchToken),
				'409': errorResponse(copy.errors.revokesItself)
			}
		}
	};
}

function uploadFilePath(copy: ApiCopy) {
	return {
		post: {
			tags: [copy.tags.token],
			operationId: 'uploadFile',
			summary: copy.endpoints.uploadFile.summary,
			description: copy.endpoints.uploadFile.description,
			parameters: [
				{ name: 'filename', in: 'query', required: false, schema: { type: 'string' } }
			],
			requestBody: {
				required: true,
				content: { 'application/octet-stream': { schema: { type: 'string', format: 'binary' } } }
			},
			responses: {
				'200': { description: copy.endpoints.uploadFile.summary },
				'400': errorResponse(copy.errors.badRequest),
				'401': errorResponse(copy.errors.unauthorized),
				'403': errorResponse(copy.errors.forbidden),
				'413': errorResponse(copy.errors.tooLarge)
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

const exampleValuesByField: Record<string, unknown> = {
	title: '3분기 결산 자료 정리',
	content: '지출 내역을 표로 정리해 주세요',
	message: '오늘 회의는 15시로 옮깁니다',
	query: '결산',
	scope: 'self',
	status: 'planned',
	size: 'M',
	startsAt: '2026-09-01',
	endsAt: '2026-09-05',
	startISO: '2026-09-01T10:00:00+09:00',
	endISO: '2026-09-01T11:00:00+09:00',
	limit: 20,
	taskHint: '3분기 결산 자료 정리',
	eventHint: '주간 회의',
	personHint: '이샘플',
	participantPersonHints: ['이샘플', '박예시'],
	recipientPersonHints: ['이샘플'],
	channelHint: '전사-공지',
	url: 'https://intern.kim/files/example.png',
	path: 'shared/reports/q3.md',
	slug: 'q3-report'
};

function exampleValueOf(name: string, schema: Record<string, unknown>): unknown {
	if (name in exampleValuesByField) return exampleValuesByField[name];
	const listed = schema.enum;
	if (Array.isArray(listed) && listed.length > 0) return listed[0];
	if (schema.type === 'integer' || schema.type === 'number') return 1;
	if (schema.type === 'boolean') return true;
	if (schema.type === 'array') return [];
	return '…';
}

// The example carries the required fields plus the ones a first call usually
// wants, so it runs as pasted instead of teasing with placeholders.
function exampleInputOf(tool: CatalogTool): Record<string, unknown> {
	const schema = (tool.inputSchema ?? {}) as {
		required?: string[];
		properties?: Record<string, Record<string, unknown>>;
	};
	const properties = schema.properties ?? {};
	const chosen = new Set(schema.required ?? []);
	for (const name of Object.keys(properties)) {
		if (name in exampleValuesByField && chosen.size < 4) chosen.add(name);
	}
	const example: Record<string, unknown> = {};
	for (const name of chosen) {
		example[name] = exampleValueOf(name, properties[name] ?? {});
	}
	return example;
}

function inputSchemaNameOf(tool: CatalogTool): string {
	return tool.name.replace(/(^|_)([a-z])/g, (_match, _gap, letter) => letter.toUpperCase()) + 'Input';
}

function successExampleOf(tool: CatalogTool): Record<string, unknown> {
	return {
		toolName: tool.name,
		outcome: 'succeeded',
		selectedBackend: 'device',
		...(tool.sideEffectClass && tool.sideEffectClass !== 'read'
			? { effects: [{ objectType: tool.namespace, effect: 'created', id: '7a93de11-…' }] }
			: {}),
		result: { '…': "the tool's own document" }
	};
}

const failureExample = {
	toolName: 'task_update',
	outcome: 'failed',
	selectedBackend: 'device',
	message: 'no task matched taskHint, and these are the closest…',
	errorCode: 'task_hint_unresolved',
	failureStage: 'target_resolution',
	retryable: true,
	result: { candidates: [{ taskID: '7a93de11-…', content: '3분기 결산 자료 정리' }] }
};

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
								input: { $ref: `#/components/schemas/${inputSchemaNameOf(tool)}` },
								...(tool.idempotency?.supported
									? { idempotencyKey: { type: 'string', description: copy.endpoints.invokeTool.idempotency } }
									: {})
							}
						},
						example: { input: exampleInputOf(tool) }
					}
				}
			},
			responses: {
				'200': {
					description: tool.name,
					content: {
						'application/json': {
							schema: { $ref: '#/components/schemas/ToolInvokeResponse' },
							examples: {
								succeeded: { value: successExampleOf(tool) },
								failed: { value: failureExample }
							}
						}
					}
				},
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
		'/tokens': listTokensPath(copy),
		'/token': tokenPath(copy),
		'/files': uploadFilePath(copy),
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

function toolInputSchemas(): Record<string, unknown> {
	const schemas: Record<string, unknown> = {};
	for (const tool of baseTools()) {
		schemas[inputSchemaNameOf(tool)] = withoutSchemaDialect(tool.inputSchema);
	}
	return schemas;
}

function createComponents(copy: ApiCopy) {
	return {
		securitySchemes: {
			memberToken: { type: 'http', scheme: 'bearer', description: copy.description }
		},
		schemas: {
			...toolInputSchemas(),
			TokenRequest: {
				type: 'object',
				properties: {
					name: { type: 'string', maxLength: 64 },
					permission: { type: 'string', enum: [...publicAPIPermissions] }
				}
			},
			TokenResponse: {
				type: 'object',
				required: ['name', 'permission', 'token'],
				properties: {
					name: { type: 'string' },
					permission: { type: 'string', enum: [...publicAPIPermissions] },
					token: { type: 'string' }
				}
			},
			TokenList: {
				type: 'object',
				required: ['tokens'],
				properties: {
					tokens: {
						type: 'array',
						items: {
							type: 'object',
							required: ['name', 'permission'],
							properties: {
								name: { type: 'string' },
								permission: { type: 'string', enum: [...publicAPIPermissions] }
							}
						}
					}
				}
			},
			TokenRevoked: {
				type: 'object',
				required: ['forgotten'],
				properties: { forgotten: { type: 'string' } }
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
				required: ['toolName', 'outcome', 'result'],
				properties: {
					toolName: { type: 'string' },
					outcome: { type: 'string', enum: ['succeeded', 'failed'] },
					selectedBackend: { type: 'string' },
					effects: {
						type: 'array',
						items: {
							type: 'object',
							properties: {
								objectType: { type: 'string' },
								effect: { type: 'string' },
								id: { type: 'string' }
							}
						}
					},
					message: { type: 'string' },
					errorCode: { type: 'string' },
					failureStage: { type: 'string' },
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
