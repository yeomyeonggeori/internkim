import { expect, test } from 'bun:test';

import { directoryChangedCapability, serveCallForMember, type Dispatch } from './forward';

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

// Somebody invited on the company is given a Linux user here, and until this
// device reads the directory again the agent refuses them as a stranger.
test('the company saying its directory changed reaches admind', async () => {
	const told: string[] = [];

	const served = await serveCallForMember(
		dispatchThatRecords(told),
		{ callID: 'call-1', capability: directoryChangedCapability, body: {} },
		'member-1'
	);

	expect(told).toEqual(['admind']);
	expect(served.status).toBe(202);
});

test('anything else still goes where it went before', async () => {
	const told: string[] = [];

	await serveCallForMember(
		dispatchThatRecords(told),
		{ callID: 'call-2', capability: 'asset.link', body: {} },
		'member-1'
	);

	expect(told).toEqual(['asset:asset.link']);
});
