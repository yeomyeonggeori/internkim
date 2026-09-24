import { describe, expect, test } from 'bun:test';
import { decisionRows, readExchange, readInboundMessages, readLLMCallRecord } from '../../../src/routes/runs/llm-calls';

const decisionRecordBody = JSON.stringify({
	kind: 'decision',
	model: 'decision-model',
	latencyMs: 812,
	decidedMessageIDs: ['message-a', 'message-b'],
	decisionAnswers: {
		'm2.reaction': { type: 'choice', choice: 'react', probabilities: { react: 0.62, none: 0.38 } },
		'm2.reactionEmoji': { type: 'choice', choice: 'tada' },
		'm2.shouldRespond': { type: 'noul', noul: 0.04 },
		'm1.shouldRespond': { type: 'noul', noul: 0.97 }
	},
	decisionDraws: { 'm2.reaction': 0.81 }
});

describe('a decision call', () => {
	test('lays out what it answered about one message, with the draw beside the chance', () => {
		const record = readLLMCallRecord(decisionRecordBody);

		if (!record) throw new Error('expected the decision record to read');
		const rows = decisionRows(record, 'm2');

		expect(rows).toEqual([
			{ question: 'reaction', answer: 'react', probability: 0.62, draw: 0.81 },
			{ question: 'reactionEmoji', answer: 'tada', probability: undefined, draw: undefined },
			{ question: 'shouldRespond', answer: 'no', probability: 0.04, draw: undefined }
		]);
	});
});

describe('the inbound messages', () => {
	test('find their own answers in a decision that judged several messages', () => {
		const [inboundMessage] = readInboundMessages([
			{
				externalMessageID: 'message-b',
				senderName: '박예시',
				promptPreview: '다음 주 출시 확정됐어요!',
				connectorStatus: 'succeeded',
				result: { ignored: true, reason: 'addressing_human dutyMatch=false' },
				decision: JSON.parse(decisionRecordBody)
			}
		]);

		expect(inboundMessage.outcome).toBe('ignored');
		expect(inboundMessage.reactionProbability).toBe(0.62);
		expect(inboundMessage.reactionDraw).toBe(0.81);
	});

	test('name the task a message became', () => {
		const [inboundMessage] = readInboundMessages([{ externalMessageID: 'message-c', connectorStatus: 'succeeded', result: { taskRunID: 'run-1' } }]);

		expect(inboundMessage.outcome).toBe('task');
		expect(inboundMessage.taskRunID).toBe('run-1');
		expect(inboundMessage.reactionProbability).toBeUndefined();
	});
});

describe('an exchange', () => {
	const requestAsSent = `{"model":"z-ai/glm-5.3","messages":[{"role":"system","content":"You are the agent."},{"role":"user","content":[{"type":"text","text":"회의록 정리해줘"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}],"tools":[{"type":"function","function":{"name":"file.read","parameters":{}}}],"response_format":{"type":"json_schema","json_schema":{"name":"bluecollar_agent_turn_action","schema":{}}},"seed":1234567}`;

	test('reads the request as sent: the messages, the tools offered and the seed', () => {
		const exchange = readExchange({
			request: requestAsSent,
			response: JSON.stringify({
				provider: 'Example',
				choices: [{ finish_reason: 'tool_calls', message: { role: 'assistant', tool_calls: [{ id: 'call-1', type: 'function', function: { name: 'file.read', arguments: '{"path":"회의록.md"}' } }] } }]
			})
		});

		expect(exchange.seed).toBe(1234567);
		expect(exchange.schemaName).toBe('bluecollar_agent_turn_action');
		expect(exchange.servedBy).toBe('Example');
		expect(exchange.toolNames).toEqual(['file.read']);
		expect(exchange.messages.map((message) => message.role)).toEqual(['system', 'user']);
		expect(exchange.messages[1]).toMatchObject({ text: '회의록 정리해줘', imageCount: 1 });
		expect(exchange.answer?.toolCalls[0]).toEqual({ name: 'file.read', arguments: '{\n  "path": "회의록.md"\n}' });
	});

	test('keeps the request bytes exactly as they were sent, for a replay to copy', () => {
		const exchange = readExchange({ request: requestAsSent });

		expect(exchange.request).toBe(requestAsSent);
		expect(exchange.answer).toBeUndefined();
	});

	test('reads a decision call as the state it was asked about and the input it was given', () => {
		const exchange = readExchange({
			request: JSON.stringify({ model: 'decision-model', state: { messages: [] }, questions: { 'm1.reaction': {}, 'm1.duty': {} } }),
			input: JSON.stringify({ conversationType: 'channel' })
		});

		expect(exchange.decisionQuestions).toEqual(['m1.duty', 'm1.reaction']);
		expect(exchange.input).toEqual({ conversationType: 'channel' });
	});
});
