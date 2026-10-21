import { afterAll, expect, test } from 'bun:test';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { carriesTheHostServiceToken } from './host-service-token';

const directory = mkdtempSync(join(tmpdir(), 'relay-host-service-token-'));
afterAll(() => rmSync(directory, { recursive: true, force: true }));
const keptAt = join(directory, 'relay-service-token');
writeFileSync(keptAt, 'kept-for-the-host-services\n');

function arrival(authorization?: string): Request {
	return new Request('http://127.0.0.1:18091/inbound', {
		method: 'POST',
		headers: authorization === undefined ? {} : { Authorization: authorization },
		body: JSON.stringify({ sender: { email: 'requester@example.com' }, message: '응 보내줘' })
	});
}

test('an arrival that does not carry the token the host services share is not taken', async () => {
	for (const authorization of [undefined, '', 'Bearer ', 'Bearer kept-for-the-host', 'kept-for-the-host-services']) {
		expect(await carriesTheHostServiceToken(arrival(authorization), keptAt)).toBe(false);
	}
});

test('an arrival from chatd, which holds the token, is taken', async () => {
	expect(await carriesTheHostServiceToken(arrival('Bearer kept-for-the-host-services'), keptAt)).toBe(true);
});

test('no arrival is taken when the token cannot be read', async () => {
	expect(await carriesTheHostServiceToken(arrival('Bearer '), join(directory, 'missing'))).toBe(false);
});
