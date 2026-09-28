import { describe, expect, test } from 'bun:test';
import { agentReplyTimeoutMs, stillWorkingSince } from '../../src/lib/components/channel/agent-working';

describe('stillWorkingSince', () => {
	const startedAt = 1_000_000;

	test('nothing is working when nothing was started', () => {
		expect(stillWorkingSince(null, startedAt)).toBeNull();
	});

	test('the agent keeps working until the reply timeout', () => {
		expect(stillWorkingSince(startedAt, startedAt + agentReplyTimeoutMs - 1)).toBe(startedAt);
	});

	test('the agent stops working once the reply timeout has passed', () => {
		expect(stillWorkingSince(startedAt, startedAt + agentReplyTimeoutMs)).toBeNull();
	});
});
