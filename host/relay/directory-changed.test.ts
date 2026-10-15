import { expect, test } from 'bun:test';

import { serveRoutedCall } from './gateway-connector';
import { directoryChangedCapability, type Dispatch } from './forward';

function dispatchThatRecords(told: string[]): Dispatch {
	return {
		serveAsset: async (capability: string) => {
			told.push(`asset:${capability}`);
			return null;
		},
		askAdmind: async () => ({ status: 200, body: null }),
		tellAdmindTheDirectoryChanged: async () => {
			told.push('admind');
			return { status: 202, body: null };
		},
		emailOfMember: async () => 'someone@example.test',
		askChatd: async () => ({ status: 200, body: null }),
		keepAttachment: async () => ({ address: '', sizeBytes: 0, digest: '' }),
		keptAlready: async () => null
	} as unknown as Dispatch;
}

const ceiling = 3_000_000;

function directoryChanged(memberID: string | null) {
	return { kind: 'call' as const, requestID: 'call-1', memberID, capability: directoryChangedCapability, body: {} };
}

test('a member saying the directory changed reaches admind', async () => {
	const told: string[] = [];

	const served = await serveRoutedCall(directoryChanged('member-1'), dispatchThatRecords(told), ceiling);

	expect(told).toEqual(['admind']);
	expect(served.status).toBe(202);
});

test('the record saying the directory changed reaches admind without naming a member', async () => {
	const told: string[] = [];

	const served = await serveRoutedCall(directoryChanged(null), dispatchThatRecords(told), ceiling);

	expect(told).toEqual(['admind']);
	expect(served.status).toBe(202);
});

test('anything else still goes where it went before', async () => {
	const told: string[] = [];

	await serveRoutedCall(
		{ kind: 'call', requestID: 'call-2', memberID: 'member-1', capability: 'asset.link', body: {} },
		dispatchThatRecords(told),
		ceiling
	);

	expect(told).toEqual(['asset:asset.link']);
});
