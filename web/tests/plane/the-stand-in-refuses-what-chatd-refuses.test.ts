import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aConnectorNobodyRuns } from './a-messenger-nobody-runs';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');
const chatdParser = join(repositoryRoot, '.dependency', 'blueclaw', 'chatd', 'src', 'outbound-parse.ts');

// A stand-in that accepts what the real one refuses hands back a green run for a
// call chatd would answer 400 to. These pin the two rules that matter, and the
// last one fails when chatd adds a third — at which point this file is the thing
// to read, not a mystery on somebody's device.

async function ask(connectorURL: string, capability: string, body: unknown): Promise<number> {
	const answer = await fetch(`${connectorURL}/v1/platform/buzz/${capability}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	return answer.status;
}

test('the stand-in refuses a direct message with no recipient', async () => {
	const connector = aConnectorNobodyRuns();
	try {
		expect(await ask(connector.url, 'dm.post', { message: '안녕하세요' })).toBe(400);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a recipient key that is not 64 hex characters', async () => {
	const connector = aConnectorNobodyRuns();
	try {
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: 'nope', message: '안녕하세요' })).toBe(400);
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: '2'.repeat(64), message: '안녕하세요' })).toBe(200);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a direct message with nothing written in it', async () => {
	const connector = aConnectorNobodyRuns();
	try {
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: '2'.repeat(64), message: '  ' })).toBe(400);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a post to no thread, channel or channel name', async () => {
	const connector = aConnectorNobodyRuns();
	try {
		expect(await ask(connector.url, 'message.post', { message: '안녕하세요' })).toBe(400);
		expect(await ask(connector.url, 'message.post', { threadID: 'buzz:room', message: '안녕하세요' })).toBe(200);
	} finally {
		connector.stop();
	}
});

test('the rules the stand-in copies are still the rules chatd has', () => {
	const contract = readFileSync(chatdParser, 'utf8');
	const parser = contract.slice(contract.indexOf('parseDirectMessagePostRequest'));
	const rules = parser.slice(0, parser.indexOf('\n}'));
	expect(
		rules.includes('[0-9a-f]{64}'),
		'chatd no longer checks the recipient key is 64 hex characters; a-messenger-nobody-runs.ts still does'
	).toBe(true);
	expect(
		rules.includes('requireString(record, "message")'),
		'chatd no longer requires a message; a-messenger-nobody-runs.ts still does'
	).toBe(true);
	expect(
		rules.includes('requireString(record, "counterpartPubkeyHex")'),
		'chatd no longer requires a recipient; a-messenger-nobody-runs.ts still does'
	).toBe(true);
	const postParser = contract.slice(contract.indexOf('parseMessagePostRequest'));
	const postRules = postParser.slice(0, postParser.indexOf('\n}'));
	expect(
		postRules.includes('message.post requires threadID, channelID, or channelName'),
		'chatd no longer requires a thread, channel or channel name on a post; a-messenger-nobody-runs.ts still does'
	).toBe(true);
});
