import { describe, expect, test } from 'bun:test';
import { seatAdminsFromDirectory } from '../../../src/lib/messenger/channel-admin-owners';
import type { MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

function directoryOf(overrides: Partial<MessengerDirectory> = {}): MessengerDirectory {
	return {
		nameOfMember: new Map([
			['admin1', '이샘플'],
			['admin2', '박예시'],
			['member1', '최견본']
		]),
		nameOfExternal: new Map(),
		memberOfExternal: new Map([
			['U1', 'admin1'],
			['U2', 'admin2'],
			['U3', 'member1']
		]),
		externalsOfMember: new Map([
			['admin1', ['U1']],
			['admin2', ['U2']],
			['member1', ['U3']]
		]),
		memberOfEmail: new Map(),
		adminMemberIDs: new Set(['admin1', 'admin2']),
		...overrides
	};
}

describe('seatAdminsFromDirectory', () => {
	test('every admin with a messenger account is seated', async () => {
		const seatedExternalIDs: string[] = [];
		const unseatedNames = await seatAdminsFromDirectory(directoryOf(), async (externalID) => {
			seatedExternalIDs.push(externalID);
		});
		expect(seatedExternalIDs).toEqual(['U1', 'U2']);
		expect(unseatedNames).toEqual([]);
	});

	test('an admin with no messenger account is skipped and reported as unseated', async () => {
		const directory = directoryOf({ externalsOfMember: new Map([['admin2', ['U2']]]) });
		const seatedExternalIDs: string[] = [];
		const unseatedNames = await seatAdminsFromDirectory(directory, async (externalID) => {
			seatedExternalIDs.push(externalID);
		});
		expect(seatedExternalIDs).toEqual(['U2']);
		expect(unseatedNames).toEqual(['이샘플']);
	});

	test("one admin's failure does not stop the others", async () => {
		const seatedExternalIDs: string[] = [];
		const unseatedNames = await seatAdminsFromDirectory(directoryOf(), async (externalID) => {
			if (externalID === 'U1') throw new Error('the relay refused this owner');
			seatedExternalIDs.push(externalID);
		});
		expect(seatedExternalIDs).toEqual(['U2']);
		expect(unseatedNames).toEqual(['이샘플']);
	});

	test('a member who is not a company admin is never seated', async () => {
		const seatedExternalIDs: string[] = [];
		await seatAdminsFromDirectory(directoryOf(), async (externalID) => {
			seatedExternalIDs.push(externalID);
		});
		expect(seatedExternalIDs).toEqual(['U1', 'U2']);
	});

	test('a directory with no admins seats nobody', async () => {
		const directory = directoryOf({ adminMemberIDs: new Set() });
		const seatedExternalIDs: string[] = [];
		const unseatedNames = await seatAdminsFromDirectory(directory, async (externalID) => {
			seatedExternalIDs.push(externalID);
		});
		expect(seatedExternalIDs).toEqual([]);
		expect(unseatedNames).toEqual([]);
	});
});
