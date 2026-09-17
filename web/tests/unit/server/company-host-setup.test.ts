import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { describe, expect, test } from 'bun:test';
import { fullPublicAPIPermission } from '../../../src/lib/public-api-permission';
import {
	callingHostAdministrator,
	createHostConfiguration
} from '../../../src/lib/server/company-host-setup';
import type { CallingMember } from '../../../src/lib/server/member-request';

const companyID = '00000000-0000-4000-8000-000000000001';
const projectURL = 'https://project.supabase.co';
const appURL = 'https://intern.example.com';

type HostData = {
	company?: Record<string, unknown>;
	existingAgent?: { last_seen_at: string | null; revoked_at: string | null };
	agentWrites: number;
};

function responseBody(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

type FetchImplementation = (
	input: Parameters<typeof fetch>[0],
	initialization?: Parameters<typeof fetch>[1]
) => ReturnType<typeof fetch>;

function supabaseClient(fetcher: typeof fetch, key: string): SupabaseClient {
	return createClient(projectURL, key, {
		auth: { autoRefreshToken: false, persistSession: false },
		global: { fetch: fetcher }
	});
}

function companyHostMember(data: HostData): CallingMember {
	const fetcher: FetchImplementation = async (request, init) => {
		const url = new URL(typeof request === 'string' ? request : request instanceof URL ? request.href : request.url);
		const method = request instanceof Request ? request.method : init?.method ?? 'GET';
		if (url.pathname.endsWith('/company')) return responseBody(data.company ?? { id: companyID, name: 'Example Co', slug: 'example' });
		if (url.pathname.endsWith('/agent') && method === 'GET') {
			return responseBody(data.existingAgent ?? null);
		}
		if (url.pathname.endsWith('/agent') && method === 'POST') {
			data.agentWrites += 1;
			return responseBody({ id: '00000000-0000-4000-8000-000000000002' });
		}
		return responseBody([]);
	};
	const supabaseFetcher = Object.assign(fetcher, { preconnect() {} });
	const caller = supabaseClient(supabaseFetcher, 'publishable-key');
	const record = supabaseClient(supabaseFetcher, 'secret-key');
	return {
		accessToken: 'member-session',
		caller,
		record,
		memberID: '00000000-0000-4000-8000-000000000003',
		companyID,
		email: 'admin@example.com',
		permission: fullPublicAPIPermission,
		tokenName: ''
	};
}

const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: 'publishable-key',
	SUPABASE_SECRET_KEY: 'secret-key',
	GATEWAY_URL: 'https://gateway.example.com'
};

describe('company host setup', () => {
	test('returns only public company host settings and a fresh key', async () => {
		const data: HostData = { agentWrites: 0 };
		const configuration = await createHostConfiguration(companyHostMember(data), {
			...environment,
			GATEWAY_ADMIN_TOKEN: 'must-not-leak'
		}, appURL, false);

		expect(configuration).toEqual({
			schemaVersion: 1,
			appURL,
			company: { id: companyID, name: 'Example Co', slug: 'example' },
			centralPlane: { projectURL, publishableKey: 'publishable-key' },
			gatewayURL: 'https://gateway.example.com',
			agentKey: expect.stringMatching(/^[a-f0-9]{64}$/)
		});
		expect(configuration).not.toHaveProperty('centralPlane.secretKey');
		expect(configuration).not.toHaveProperty('gatewayAdminToken');
		expect(JSON.stringify(configuration)).not.toContain('secret-key');
		expect(JSON.stringify(configuration)).not.toContain('must-not-leak');
		expect(data.agentWrites).toBe(1);
	});

	test('requires the gateway before issuing an agent key', async () => {
		const data: HostData = { agentWrites: 0 };
		const attempt = createHostConfiguration(companyHostMember(data), {
			SUPABASE_URL: projectURL,
			SUPABASE_PUBLISHABLE_KEY: 'publishable-key',
			SUPABASE_SECRET_KEY: 'secret-key'
		}, appURL, false);

		await expect(attempt).rejects.toMatchObject({ status: 503 });
		expect(data.agentWrites).toBe(0);
	});

	test('requires explicit confirmation before replacing an existing connection', async () => {
		const data: HostData = { existingAgent: { last_seen_at: null, revoked_at: null }, agentWrites: 0 };

		await expect(createHostConfiguration(companyHostMember(data), environment, appURL, false))
			.rejects.toMatchObject({ status: 409 });
		expect(data.agentWrites).toBe(0);
	});

	test('issues a replacement key after confirmation', async () => {
		const data: HostData = { existingAgent: { last_seen_at: '2026-09-06T00:00:00Z', revoked_at: null }, agentWrites: 0 };
		const configuration = await createHostConfiguration(companyHostMember(data), environment, appURL, true);

		expect(configuration.agentKey).toMatch(/^[a-f0-9]{64}$/);
		expect(data.agentWrites).toBe(1);
	});

	test('rejects an injected company field from the strict host schema', async () => {
		const data: HostData = {
			company: { id: companyID, name: 'Example Co', slug: 'example', injectedCompanyID: 'other' },
			agentWrites: 0
		};

		await expect(createHostConfiguration(companyHostMember(data), environment, appURL, false))
			.rejects.toMatchObject({ status: 503 });
		expect(data.agentWrites).toBe(0);
	});
});

describe('company host administrator authorization', () => {
	function requestWith(member: CallingMember): Request {
		return new Request('https://intern.example.com/api/company/host-setup', { headers: { authorization: 'Bearer member-session' } });
	}

	test('rejects a personal access token before host setup can issue a key', async () => {
		const member = companyHostMember({ agentWrites: 0 });
		member.tokenName = 'automation';

		await expect(callingHostAdministrator(requestWith(member), environment, async () => member))
			.rejects.toMatchObject({ status: 403 });
	});

	test('rejects an ordinary member', async () => {
		const member = companyHostMember({ agentWrites: 0 });
		const fetcher = Object.assign(async (): Promise<Response> => responseBody([{ is_admin: false, status: 'active' }]), { preconnect() {} });
		const resolved = { ...member, caller: supabaseClient(fetcher, 'publishable-key') };

		await expect(callingHostAdministrator(requestWith(resolved), environment, async () => resolved))
			.rejects.toMatchObject({ status: 403 });
	});

	test('rejects an inactive administrator', async () => {
		const member = companyHostMember({ agentWrites: 0 });
		const fetcher = Object.assign(async (): Promise<Response> => responseBody([{ is_admin: true, status: 'invited' }]), { preconnect() {} });
		const resolved = { ...member, caller: supabaseClient(fetcher, 'publishable-key') };

		await expect(callingHostAdministrator(requestWith(resolved), environment, async () => resolved))
			.rejects.toMatchObject({ status: 403 });
	});
});
