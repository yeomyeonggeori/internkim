import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { tell, tellCapability, type Telling } from '../../src/lib/server/tell';
import type { CompanyCallTransport } from '../../src/lib/server/public-api/company-call';

const environment = {
	GATEWAY_URL: 'wss://gateway.example.test',
	GATEWAY_ADMIN_TOKEN: 'an-admin-token',
	SUPABASE_URL: 'https://ours.supabase.co',
	SUPABASE_SECRET_KEY: 'a-service-key'
};

const telling: Telling = {
	memberID: 'member-1',
	category: 'approval',
	title: '결재 요청',
	body: '이샘플님의 연차 신청이 기다립니다'
};

const tellMemberURL = 'https://ours.supabase.co/functions/v1/tell-member';

type MemberRow = { company_id: string | null; email: string | null };

function recordWhere(member: MemberRow | null): SupabaseClient {
	return {
		from: () => ({
			select: () => ({ eq: () => ({ maybeSingle: async () => ({ data: member, error: null }) }) })
		})
	} as unknown as SupabaseClient;
}

type Sent = { url: string; authorization: string; body: Record<string, unknown> };

type Answers = { push?: { status: number; reached?: number; error?: string }; gateway?: number };

function transportRecording(sent: Sent[], answers: Answers = {}): CompanyCallTransport {
	return async (url, options) => {
		const authorization = (options.headers as Record<string, string>)?.Authorization ?? '';
		const body = JSON.parse(options.body) as Record<string, unknown>;
		sent.push({ url, authorization, body });

		if (url === tellMemberURL) {
			const push = answers.push ?? { status: 200, reached: 0 };
			const said = push.error ? { error: push.error } : { reached: push.reached ?? 0, pruned: 0 };
			return { status: push.status, json: async () => said };
		}
		const status = answers.gateway ?? 200;
		return { status: 200, json: async () => ({ status, body: { delivered: status < 300 } }) };
	};
}

function pushesOf(sent: Sent[]): Sent[] {
	return sent.filter((one) => one.url === tellMemberURL);
}

function gatewayCallsOf(sent: Sent[]): Sent[] {
	return sent.filter((one) => one.url !== tellMemberURL);
}

describe('tell', () => {
	test('asks the company to write the person a direct message, naming their address', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(environment, telling, record, transportRecording(sent));

		expect(told.messaged).toBe(true);
		expect(told.failure).toBeUndefined();

		const calls = gatewayCallsOf(sent);
		expect(calls).toHaveLength(1);
		expect(calls[0]?.url).toBe('https://gateway.example.test/company/company-1/call');
		expect((calls[0]?.body as { capability?: string }).capability).toBe(tellCapability);
		expect((calls[0]?.body as { body?: unknown }).body).toEqual({
			recipientEmail: 'sample@example.test',
			message: '결재 요청\n이샘플님의 연차 신청이 기다립니다'
		});
	});

	test('the push goes to the project, signed with the key only the project holds', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		await tell(environment, telling, record, transportRecording(sent, { push: { status: 200, reached: 1 } }));

		const pushes = pushesOf(sent);
		expect(pushes).toHaveLength(1);
		expect(pushes[0]?.authorization).toBe('Bearer a-service-key');
		expect(pushes[0]?.body).toEqual({
			memberID: 'member-1',
			category: 'approval',
			title: '결재 요청',
			body: '이샘플님의 연차 신청이 기다립니다',
			openPath: '/attendance/'
		});
	});

	test('the push names the member who asked, so the project can show their picture', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		await tell(environment, { ...telling, senderMemberID: 'member-2' }, record, transportRecording(sent));

		expect(pushesOf(sent)[0]?.body).toMatchObject({ memberID: 'member-1', senderMemberID: 'member-2' });
	});

	test('a person no device is subscribed for is not a failure, only a person no push reached', async () => {
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(environment, telling, record, transportRecording([]));

		expect(told).toEqual({ pushed: false, messaged: true });
	});

	test('a push that reaches a device is reported as pushed', async () => {
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(
			environment,
			telling,
			record,
			transportRecording([], { push: { status: 200, reached: 2 } })
		);

		expect(told).toEqual({ pushed: true, messaged: true });
	});

	test('the messenger still hears it when the project refuses the push', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(
			environment,
			telling,
			record,
			transportRecording(sent, { push: { status: 503, error: 'this deployment has no VAPID keys' } })
		);

		expect(told.messaged).toBe(true);
		expect(told.pushed).toBe(false);
		expect(told.failure).toBe('web push: this deployment has no VAPID keys');
		expect(gatewayCallsOf(sent)).toHaveLength(1);
	});

	test('a refusal the project does not explain still names the status', async () => {
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(environment, telling, record, transportRecording([], { push: { status: 500 } }));

		expect(told.failure).toBe('web push: the project answered 500');
	});

	test('the push is still tried when the company refuses the direct message', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(environment, telling, record, transportRecording(sent, { gateway: 503 }));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: the company answered 503');
		expect(pushesOf(sent)).toHaveLength(1);
	});

	test('a deployment with no project configured says so and still writes the person', async () => {
		const sent: Sent[] = [];
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell(
			{ ...environment, SUPABASE_SECRET_KEY: '' },
			telling,
			record,
			transportRecording(sent)
		);

		expect(told).toEqual({
			pushed: false,
			messaged: true,
			failure: 'web push: the project is not configured'
		});
		expect(pushesOf(sent)).toHaveLength(0);
		expect(gatewayCallsOf(sent)).toHaveLength(1);
	});

	test('a member with no company and no address is reported rather than silently skipped', async () => {
		const told = await tell(environment, telling, recordWhere(null), transportRecording([]));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: member member-1 has no company and no address');
	});

	test('a plane with no gateway configured fails the messenger alone', async () => {
		const record = recordWhere({ company_id: 'company-1', email: 'sample@example.test' });

		const told = await tell({ ...environment, GATEWAY_URL: '' }, telling, record, transportRecording([]));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: the company gateway is not configured');
	});

	test('a plane with no control plane credentials tells nobody and says which', async () => {
		const told = await tell({ ...environment, SUPABASE_URL: '' }, telling);

		expect(told).toEqual({ pushed: false, messaged: false, failure: 'the control plane is not configured' });
	});
});
