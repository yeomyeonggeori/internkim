// An OpenAI-compatible endpoint that answers instantly and costs nothing. The
// question a plane run asks about the model is not what it says: it is whether
// the agent reaches the endpoint the plane rendered for its tier, with the model
// that tier was given and the key that endpoint's file holds. So this records
// the ask and answers the shape a caller of /chat/completions expects.

export type RecordedCompletion = {
	model: string;
	authorization: string | null;
	toolChoice: string | null;
};

export type AModelNobodyPaysFor = {
	url: string;
	completions: RecordedCompletion[];
	modelsAsked: () => string[];
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

// The one model client asks for structured output as a forced tool call named
// after the schema, so an answer it can read is a tool call by that same name.
function answerFor(schemaName: string | null): Response {
	if (schemaName === null) {
		return Response.json({
			choices: [{ message: { role: 'assistant', content: 'nothing to do' }, finish_reason: 'stop' }],
			usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 }
		});
	}
	return Response.json({
		choices: [
			{
				message: {
					role: 'assistant',
					tool_calls: [
						{
							id: 'call-1',
							type: 'function',
							function: { name: schemaName, arguments: '{}' }
						}
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
			completions.push({
				model: typeof document.model === 'string' ? document.model : '',
				authorization: request.headers.get('authorization'),
				toolChoice: toolChoiceName(document)
			});
			return answerFor(toolChoiceName(document));
		}
	});
	return {
		url: `http://127.0.0.1:${server.port}/v1`,
		completions,
		modelsAsked: () => completions.map((completion) => completion.model),
		stop: () => server.stop(true)
	};
}
