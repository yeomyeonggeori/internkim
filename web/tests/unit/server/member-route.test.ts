import { describe, expect, test } from 'bun:test';
import { memberWriteSchema, saveMember, withdrawMember } from '../../../src/lib/server/member-directory';
import type { CompanyDirectory } from '../../../src/lib/server/member-directory';

type MemberRow = {
	id: string;
	company_id: string;
	email: string;
	name: string | null;
	note: string | null;
	is_admin: boolean;
	status: string;
	messenger: Record<string, string> | null;
};

type Written = { kind: 'update' | 'insert'; row: Record<string, unknown> };

function directoryHolding(rows: MemberRow[]): { directory: CompanyDirectory; written: Written[] } {
	const written: Written[] = [];
	let filters: Record<string, unknown> = {};
	const matching = () =>
		rows.filter((row) => Object.entries(filters).every(([column, value]) => row[column as keyof MemberRow] === value));
	const savedRow = (row: Record<string, unknown>, id: string) => ({
		select: () => ({
			single: async () => ({ data: { id, name: row.name, is_admin: row.is_admin, status: 'active' }, error: null })
		})
	});
	const query = () => {
		const chain = {
			eq: (column: string, value: unknown) => {
				filters = { ...filters, [column]: value };
				return chain;
			},
			neq: () => chain,
			order: () => chain,
			returns: async () => ({ data: matching(), error: null }),
			maybeSingle: async () => ({ data: matching()[0] ?? null, error: null })
		};
		return chain;
	};
	const client = {
		from: () => ({
			select: () => {
				filters = {};
				return query();
			},
			update: (row: Record<string, unknown>) => {
				written.push({ kind: 'update', row });
				return { eq: (_column: string, id: string) => ({ ...savedRow(row, id), error: null }) };
			},
			insert: (row: Record<string, unknown>) => {
				written.push({ kind: 'insert', row });
				return savedRow(row, 'member-new');
			}
		})
	};
	return { directory: { client, companyID: 'company-1' } as unknown as CompanyDirectory, written };
}

const held: MemberRow = {
	id: 'member-1',
	company_id: 'company-1',
	email: 'member@example.com',
	name: '이샘플',
	note: 'HR compensation follow-up',
	is_admin: true,
	status: 'active',
	messenger: { buzz: 'a-buzz-key' }
};

describe('the member door keeps what a write did not name', () => {
	test('a write that names only the name does not demote', async () => {
		const { directory, written } = directoryHolding([held]);

		const saved = await saveMember(directory, { email: 'member@example.com', name: '이샘플' });

		expect(written[0].kind).toBe('update');
		expect(written[0].row).toMatchObject({ name: '이샘플', is_admin: true, note: 'HR compensation follow-up' });
		expect(saved).toMatchObject({ memberID: 'member-1', email: 'member@example.com', role: 'admin' });
	});

	test('a write that names only the role does not blank the name', async () => {
		const { directory, written } = directoryHolding([held]);

		await saveMember(directory, { email: 'member@example.com', role: 'member' });

		expect(written[0].row).toMatchObject({ name: '이샘플', is_admin: false });
	});

	test('messenger accounts merge with the ones already held', async () => {
		const { directory, written } = directoryHolding([held]);

		await saveMember(directory, { email: 'member@example.com', messenger: { slack: 'U123', empty: ' ' } });

		expect(written[0].row.messenger).toEqual({ buzz: 'a-buzz-key', slack: 'U123' });
	});

	test('an address nobody holds becomes a member with what was offered', async () => {
		const { directory, written } = directoryHolding([]);

		const saved = await saveMember(directory, { email: 'new@example.com', name: '박예시' });

		expect(written[0].kind).toBe('insert');
		expect(written[0].row).toMatchObject({
			company_id: 'company-1',
			email: 'new@example.com',
			name: '박예시',
			is_admin: false
		});
		expect(saved.memberID).toBe('member-new');
	});

	test('an address held by another company is refused, not seated twice', async () => {
		const { directory, written } = directoryHolding([{ ...held, company_id: 'company-2' }]);

		const refusal = await saveMember(directory, { email: 'member@example.com', name: '이샘플' }).catch(
			(thrown: { status?: number }) => thrown
		);

		expect((refusal as { status?: number }).status).toBe(409);
		expect(written).toHaveLength(0);
	});
});

describe('withdrawing keeps the row', () => {
	test('only the status changes', async () => {
		const { directory, written } = directoryHolding([held]);

		const withdrawn = await withdrawMember(directory, 'member@example.com');

		expect(withdrawn).toEqual({ memberID: 'member-1', email: 'member@example.com', status: 'withdrawn' });
		expect(written).toEqual([{ kind: 'update', row: { status: 'withdrawn' } }]);
	});

	test('an address nobody here goes by is nobody', async () => {
		const { directory, written } = directoryHolding([]);

		expect(await withdrawMember(directory, 'stranger@example.com')).toBeNull();
		expect(written).toHaveLength(0);
	});
});

describe('what a member write may say', () => {
	test('normalizes the address and accepts the account fields', () => {
		const asked = memberWriteSchema.safeParse({ email: ' Member@Example.com ', role: 'admin', note: ' a note ' });

		expect(asked.success).toBe(true);
		expect(asked.data).toEqual({ email: 'member@example.com', role: 'admin', note: 'a note' });
	});

	test('refuses a role the company does not have and a missing address', () => {
		expect(memberWriteSchema.safeParse({ email: 'member@example.com', role: 'owner' }).success).toBe(false);
		expect(memberWriteSchema.safeParse({ name: '이샘플' }).success).toBe(false);
	});
});
