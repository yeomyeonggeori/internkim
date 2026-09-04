// An OpenAI-compatible endpoint that answers instantly and costs nothing. The
// question a plane run asks about the model is not what it says: it is whether
// the agent reaches the endpoint the plane rendered for its tier, with the model
// that tier was given and the key that endpoint's file holds. So this records
// the ask and answers the shape a caller of /chat/completions expects.

export type RecordedCompletion = {
	model: string;
	authorization: string | null;
	toolChoice: string | null;
	/** The action step offers tools and lets the model choose; this is what it chose. */
	answeredWith: string | null;
};

export type AModelNobodyPaysFor = {
	url: string;
	completions: RecordedCompletion[];
	modelsAsked: () => string[];
	/** Queue what the model answers next for one schema, in the order asked. */
	answerNext: (schemaName: string, document: string) => void;
	/** Queue the next agent action, which the loop takes as a native tool call. */
	callNext: (toolName: string, argumentsDocument: string) => void;
	stop: () => void;
};

function toolChoiceName(document: Record<string, unknown>): string | null {
	const toolChoice = document.tool_choice;
	if (typeof toolChoice !== 'object' || toolChoice === null) return null;
	const chosenFunction = (toolChoice as Record<string, unknown>).function;
	if (typeof chosenFunction !== 'object' || chosenFunction === null) return null;
	const name = (chosenFunction as Record<string, unknown>).name;
	return typeof name === 'string' ? name : null;
}

function nothingToDo(): Response {
	return Response.json({
		choices: [{ message: { role: 'assistant', content: 'nothing to do' }, finish_reason: 'stop' }],
		usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 }
	});
}

// The one model client asks for structured output as a forced tool call named
// after the schema, and asks for an agent action as a free choice among tools
// named after the actions. Either way an answer it can read is a tool call.
function aToolCall(name: string, argumentsDocument: string): Response {
	return Response.json({
		choices: [
			{
				message: {
					role: 'assistant',
					tool_calls: [
						{ id: `call-${name}`, type: 'function', function: { name, arguments: argumentsDocument } }
					]
				},
				finish_reason: 'tool_calls'
			}
		],
		usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 }
	});
}

export function aModelNobodyPaysFor(): AModelNobodyPaysFor {
	const completions: RecordedCompletion[] = [];
	// A wiring scenario asks nobody to think, so an unscripted schema still gets
	// an empty document. A scenario that needs the agent to decide something
	// queues the decision instead of paying a model for it.
	const scripted = new Map<string, string[]>();
	const nextActions: { toolName: string; argumentsDocument: string }[] = [];
	const server = Bun.serve({
		port: 0,
		async fetch(request) {
			const path = new URL(request.url).pathname;
			if (path.endsWith('/embeddings')) {
				return Response.json({ data: [{ embedding: [0, 0, 0] }] });
			}
			if (!path.endsWith('/chat/completions')) {
				return new Response(`no route for ${path}`, { status: 404 });
			}
			const document = (await request.json()) as Record<string, unknown>;
			const schemaName = toolChoiceName(document);
			const nextAction = schemaName === null && Array.isArray(document.tools) ? nextActions.shift() : undefined;
			completions.push({
				model: typeof document.model === 'string' ? document.model : '',
				authorization: request.headers.get('authorization'),
				toolChoice: schemaName,
				answeredWith: schemaName ?? nextAction?.toolName ?? null
			});
			if (schemaName !== null) {
				return aToolCall(schemaName, scripted.get(schemaName)?.shift() ?? '{}');
			}
			if (nextAction) {
				return aToolCall(nextAction.toolName, nextAction.argumentsDocument);
			}
			return nothingToDo();
		}
	});
	return {
		url: `http://127.0.0.1:${server.port}/v1`,
		completions,
		modelsAsked: () => completions.map((completion) => completion.model),
		answerNext: (schemaName: string, document: string) => {
			scripted.set(schemaName, [...(scripted.get(schemaName) ?? []), document]);
		},
		callNext: (toolName: string, argumentsDocument: string) => {
			nextActions.push({ toolName, argumentsDocument });
		},
		stop: () => server.stop(true)
	};
}
