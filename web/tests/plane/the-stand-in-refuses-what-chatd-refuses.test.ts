import { expect, test } from 'bun:test';
import { aConnectorNobodyRuns } from './a-messenger-nobody-runs';

const serviceToken = 'c'.repeat(64);

async function ask(connectorURL: string, capability: string, body: unknown, authorization = `Bearer ${serviceToken}`): Promise<number> {
	const answer = await fetch(`${connectorURL}/v1/platform/buzz/${capability}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: authorization },
		body: JSON.stringify(body)
	});
	return answer.status;
}

test('the stand-in refuses a post from anything that does not hold the service token', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'message.post', { threadID: 'buzz:room', message: '안녕하세요' }, '')).toBe(401);
		expect(await ask(connector.url, 'dm.send', { threadID: 'buzz:room', message: '안녕하세요' }, `Bearer ${'d'.repeat(64)}`)).toBe(401);
		expect(await ask(connector.url, 'message.post', { threadID: 'buzz:room', message: '안녕하세요' })).toBe(200);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a direct message with no recipient', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'dm.post', { message: '안녕하세요' })).toBe(400);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a recipient key that is not 64 hex characters', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: 'nope', message: '안녕하세요' })).toBe(400);
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: '2'.repeat(64), message: '안녕하세요' })).toBe(200);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a direct message with nothing written in it', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'dm.post', { counterpartPubkeyHex: '2'.repeat(64), message: '  ' })).toBe(400);
	} finally {
		connector.stop();
	}
});

test('the stand-in refuses a post to no thread, channel or channel name', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'message.post', { message: '안녕하세요' })).toBe(400);
		expect(await ask(connector.url, 'message.post', { threadID: 'buzz:room', message: '안녕하세요' })).toBe(200);
	} finally {
		connector.stop();
	}
});

test('a standalone attachment accepted by chatd is accepted by the stand-in', async () => {
	const connector = aConnectorNobodyRuns(serviceToken);
	try {
		expect(await ask(connector.url, 'message.post', {
			threadID: 'buzz:room', message: '',
			attachments: [{ address: 'https://example.com/sample.txt', filename: 'sample.txt', contentType: 'text/plain', sizeBytes: 6, digest: new Bun.CryptoHasher('sha256').update('sample').digest('hex') }]
		})).toBe(200);
	} finally {
		connector.stop();
	}
});
