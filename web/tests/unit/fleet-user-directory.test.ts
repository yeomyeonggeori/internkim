import { describe, expect, test } from 'bun:test';
import { fleetUserRecords, saveFleetUserRecord } from '../../src/lib/server/fleet-user-directory';
import type { FleetDirectory } from '../../src/lib/server/fleet-user-directory';

type MemberRow = {
	id: string;
	email: string | null;
	name: string | null;
	note: string | null;
	is_admin: boolean;
	status: string;
	messenger: Record<string, string> | null;
};

function directoryHolding(rows: MemberRow[]): { directory: FleetDirectory; written: Record<string, unknown>[] } {
	const written: Record<string, unknown>[] = [];
	const query = (matched: MemberRow[]) => {
		const chain = {
			eq: () => chain,
			neq: () => chain,
			order: () => chain,
			returns: async () => ({ data: matched, error: null }),
			maybeSingle: async () => ({ data: matched[0] ?? null, error: null }),
			then: undefined
		};
		return chain;
	};
	const client = {
		from: () => ({
			select: (_columns: string) => query(rows),
			update: (row: Record<string, unknown>) => {
				written.push(row);
				return { eq: async () => ({ error: null }) };
			},
			insert: async (row: Record<string, unknown>) => {
				written.push(row);
				return { error: null };
			}
		})
	};
	return { directory: { client, companyID: 'company-1' } as unknown as FleetDirectory, written };
}

describe('the fleet user list is the company directory', () => {
	test('reads people from member, carrying the note and the messenger account', async () => {
		const { directory } = directoryHolding([
			{
				id: 'member-1',
				email: 'Member@Example.com',
				name: '이샘플',
				note: 'HR compensation follow-up',
				is_admin: true,
				status: 'active',
				messenger: { mattermost: 'user-1', mattermostUsername: 'member' }
			}
		]);

		const records = await fleetUserRecords(directory);

		expect(records).toHaveLength(1);
		expect(records[0]).toMatchObject({
			memberID: 'member-1',
			email: 'member@example.com',
			name: '이샘플',
			note: 'HR compensation follow-up',
			role: 'admin',
			handle: 'member',
			mattermostUserID: 'user-1'
		});
	});

	test('every person carries the member id the device resolves them by', async () => {
		const { directory } = directoryHolding([
			{ id: 'member-3', email: 'named@example.com', name: '박예시', note: null, is_admin: false, status: 'active', messenger: null }
		]);

		const records = await fleetUserRecords(directory);

		expect(records[0].memberID).toBe('member-3');
	});

	test('a person with no messenger account is still named', async () => {
		const { directory } = directoryHolding([
			{ id: 'member-2', email: 'nobody@example.com', name: null, note: null, is_admin: false, status: 'invited', messenger: null }
		]);

		const records = await fleetUserRecords(directory);

		expect(records[0].handle).toBe('nobody');
		expect(records[0].isIncomplete).toBe(true);
		expect(records[0].role).toBe('member');
	});

	test('saving a person writes the note and the messenger account onto their member row', async () => {
		const { directory, written } = directoryHolding([
			{ id: 'member-1', email: 'member@example.com', name: '이샘플', note: null, is_admin: false, status: 'active', messenger: null }
		]);

		await saveFleetUserRecord(directory, {
			handle: 'member',
			name: '이샘플',
			email: 'member@example.com',
			note: 'HR compensation follow-up',
			role: 'admin',
			mattermostUserID: 'user-1'
		});

		expect(written[0]).toMatchObject({
			email: 'member@example.com',
			note: 'HR compensation follow-up',
			is_admin: true,
			messenger: { mattermost: 'user-1', mattermostUsername: 'member' }
		});
	});
});
