import { describe, expect, mock, test } from 'bun:test';

type Answer = { data: unknown; error: { message: string } | null };

const tablesTheDirectoryReads = ['member', 'team', 'circle_member'];
const asked = new Set<string>();
let allAsked: () => void = () => undefined;
const everyTableAsked = new Promise<void>((settle) => (allAsked = settle));

const rowsOf: Record<string, unknown[]> = {
	member: [{ id: 'member-1', email: 'member1@example.com', name: '이샘플', is_admin: false, status: 'active', team_id: 'team-1' }],
	team: [{ id: 'team-1', name: 'Sales' }],
	circle_member: [{ member_id: 'member-1', circle_id: 'circle-1' }]
};

function answerOnceEveryTableIsAsked(table: string): Promise<Answer> {
	asked.add(table);
	if (tablesTheDirectoryReads.every((name) => asked.has(name))) allAsked();
	const refusal = new Promise<Answer>((settle) =>
		setTimeout(() => settle({ data: null, error: { message: `${table} waited for a read nobody started` } }), 1000)
	);
	return Promise.race([everyTableAsked.then(() => ({ data: rowsOf[table], error: null })), refusal]);
}

function queryOf(table: string) {
	const query = {
		select: () => query,
		eq: () => query,
		order: () => query,
		returns: () => query,
		then: (resolve: (answer: Answer) => unknown, reject?: (reason: unknown) => unknown) =>
			answerOnceEveryTableIsAsked(table).then(resolve, reject)
	};
	return query;
}

mock.module('$lib/server/agent-request', () => ({
	environmentOf: () => ({}),
	callingAgent: async () => ({ client: { from: queryOf }, companyID: 'company-1' })
}));

const { GET } = await import('../../../src/routes/api/agent/member/+server');

describe('the directory a company computer reads', () => {
	test('asks for members, teams and circles at once', async () => {
		const response = await GET({
			request: new Request('https://app.example.test/api/agent/member'),
			url: new URL('https://app.example.test/api/agent/member'),
			platform: undefined
		} as never);

		expect(response.status).toBe(200);
		const answer = (await response.json()) as { members: { email: string; teamName: string; circles: string[] }[] };
		expect(answer.members).toEqual([
			expect.objectContaining({ email: 'member1@example.com', teamName: 'Sales', circles: ['circle-1'] })
		]);
	});
});
