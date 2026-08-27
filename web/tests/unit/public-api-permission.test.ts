import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';
import { publicAPIPermissions } from '../../src/lib/public-api-permission';

const gateway = readFileSync(
	join(import.meta.dir, '..', '..', '..', 'internal', 'admind', 'public_tool_gateway.go'),
	'utf8'
);

function goConstant(name: string): string {
	const declared = new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`).exec(gateway);
	if (!declared) throw new Error(`public_tool_gateway.go no longer declares ${name}`);
	return declared[1];
}

function permissionsAdmindKnows(): string[] {
	const returned = /func knownPublicAPIPermissions\(\) \[\]string \{\s*return \[\]string\{([^}]*)\}/.exec(gateway);
	if (!returned) throw new Error('knownPublicAPIPermissions no longer returns a literal slice');
	return returned[1]
		.split(',')
		.map((entry) => entry.trim())
		.filter((entry) => entry.length > 0)
		.map(goConstant);
}

describe('the permissions a key may carry are the list admind keeps', () => {
	test('the web copy names exactly what knownPublicAPIPermissions returns, in order', () => {
		expect([...publicAPIPermissions]).toEqual(permissionsAdmindKnows());
	});

	test('the ladder admind reads is the one this app offers', () => {
		expect(permissionsAdmindKnows()).toEqual(['read', 'write', 'delete']);
	});
});
