import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { tell, tellCapability, type Telling } from '../../src/lib/server/tell';
import type { CompanyCallTransport } from '../../src/lib/server/public-api/company-call';

const environment = {
	GATEWAY_URL: 'wss://gateway.example.test',
	GATEWAY_ADMIN_TOKEN: 'an-admin-token',
	VAPID_PUBLIC_KEY: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	VAPID_PRIVATE_KEY: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	VAPID_SUBJECT: 'mailto:support@example.com'
};

const telling: Telling = {
	memberID: 'member-1',
	category: 'approval',
	title: '결재 요청',
	body: '이샘플님의 연차 신청이 기다립니다'
};

type MemberRow = { company_id: string | null; email: string | null };

function recordWhere(options: { member?: MemberRow | null; settingsFailure?: string; reads?: string[] }): SupabaseClient {
	const reads = options.reads ?? [];
	return {
		from(table: string) {
			if (table === 'member') {
				return {
					select: (columns: string) => {
						reads.push(columns);
						return {
							eq: () => ({
								maybeSingle: async () => {
									if (columns !== 'notification_settings') {
										return { data: options.member ?? null, error: null };
									}
									if (options.settingsFailure) return { data: null, error: { message: options.settingsFailure } };
									return { data: { notification_settings: null }, error: null };
								}
							})
						};
					}
				};
			}
			return {
				select: () => ({ eq: () => ({ eq: () => ({ returns: async () => ({ data: [], error: null }) }) }) }),
				delete: () => ({ eq: async () => ({ error: null }) })
			};
		}
	} as unknown as SupabaseClient;
}

type CompanyCallRecord = { url: string; capability: string; body: Record<string, unknown> };

function transportAnswering(status: number, calls: CompanyCallRecord[]): CompanyCallTransport {
	return async (url, options) => {
		const sent = JSON.parse(options.body) as { capability: string; body: Record<string, unknown> };
		calls.push({ url, capability: sent.capability, body: sent.body });
		return { status: 200, json: async () => ({ status, body: { delivered: status < 300 } }) };
	};
}

describe('tell', () => {
	test('asks the company to write the person a direct message, naming their address', async () => {
		const calls: CompanyCallRecord[] = [];
		const record = recordWhere({ member: { company_id: 'company-1', email: 'sample@example.test' } });

		const told = await tell(environment, telling, record, transportAnswering(200, calls));

		expect(told.messaged).toBe(true);
		expect(told.failure).toBeUndefined();
		expect(calls).toHaveLength(1);
		expect(calls[0]?.url).toBe('https://gateway.example.test/company/company-1/call');
		expect(calls[0]?.capability).toBe(tellCapability);
		expect(calls[0]?.body).toEqual({
			recipientEmail: 'sample@example.test',
			message: '결재 요청\n이샘플님의 연차 신청이 기다립니다'
		});
	});

	test('a person no device is subscribed for is not a failure, only a person no push reached', async () => {
		const record = recordWhere({ member: { company_id: 'company-1', email: 'sample@example.test' } });

		const told = await tell(environment, telling, record, transportAnswering(200, []));

		expect(told).toEqual({ pushed: false, messaged: true });
	});

	test('the messenger still hears it when the push channel throws', async () => {
		const calls: CompanyCallRecord[] = [];
		const record = recordWhere({
			member: { company_id: 'company-1', email: 'sample@example.test' },
			settingsFailure: 'the notification settings are out of reach'
		});

		const told = await tell(environment, telling, record, transportAnswering(200, calls));

		expect(told.messaged).toBe(true);
		expect(told.pushed).toBe(false);
		expect(told.failure).toBe('web push: the notification settings are out of reach');
		expect(calls).toHaveLength(1);
	});

	test('the push channel is still tried when the company refuses the direct message', async () => {
		const reads: string[] = [];
		const record = recordWhere({ member: { company_id: 'company-1', email: 'sample@example.test' }, reads });

		const told = await tell(environment, telling, record, transportAnswering(503, []));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: the company answered 503');
		expect(reads).toContain('notification_settings');
	});

	test('a deployment with no push keys says so and still writes the person', async () => {
		const calls: CompanyCallRecord[] = [];
		const record = recordWhere({ member: { company_id: 'company-1', email: 'sample@example.test' } });

		const told = await tell(
			{ ...environment, VAPID_PRIVATE_KEY: '' },
			telling,
			record,
			transportAnswering(200, calls)
		);

		expect(told).toEqual({
			pushed: false,
			messaged: true,
			failure: 'web push: this deployment holds no web push keys'
		});
		expect(calls).toHaveLength(1);
	});

	test('a member with no company and no address is reported rather than silently skipped', async () => {
		const record = recordWhere({ member: null });

		const told = await tell(environment, telling, record, transportAnswering(200, []));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: member member-1 has no company and no address');
	});

	test('a plane with no gateway configured fails the messenger alone', async () => {
		const record = recordWhere({ member: { company_id: 'company-1', email: 'sample@example.test' } });

		const told = await tell({ ...environment, GATEWAY_URL: '' }, telling, record, transportAnswering(200, []));

		expect(told.messaged).toBe(false);
		expect(told.failure).toBe('direct message: the company gateway is not configured');
	});

	test('a plane with no control plane credentials tells nobody and says which', async () => {
		const told = await tell({ ...environment, SUPABASE_URL: '' }, telling);

		expect(told).toEqual({ pushed: false, messaged: false, failure: 'the control plane is not configured' });
	});
});
