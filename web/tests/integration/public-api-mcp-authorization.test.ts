import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import { UnauthorizedError, type OAuthClientProvider } from '@modelcontextprotocol/sdk/client/auth.js';
import type {
	OAuthClientInformationMixed,
	OAuthClientMetadata,
	OAuthTokens
} from '@modelcontextprotocol/sdk/shared/auth.js';
import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { addMember, controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: reachMCP } = await import('../../src/routes/api/v1/mcp/+server');
const { GET: readResourceMetadata } = await import(
	'../../src/routes/.well-known/oauth-protected-resource/[...resource]/+server'
);

const networkHookTimeout = 60_000;
const record = controlPlane({ projectURL, serviceRoleKey });
const slug = `mcp-authorization-${Date.now()}`;
const memberEmail = `${slug}-member@example.test`;
const memberPassword = `${slug}-password`;
const planeOrigin = 'https://api.example.test';
const toolServer = new URL(`${planeOrigin}/v1/mcp`);

let companyID = '';
const registeredClientIDs: string[] = [];

beforeAll(async () => {
	const provisioned = await provisionCompany(
		record,
		{ name: 'MCP Authorization Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	const memberID = await addMember(record, companyID, memberEmail);
	const { data: account } = await record.auth.admin.createUser({
		email: memberEmail,
		password: memberPassword,
		email_confirm: true
	});
	await record.from('member').update({ user_id: account.user?.id, status: 'active' }).eq('id', memberID);
}, networkHookTimeout);

afterAll(async () => {
	for (const clientID of registeredClientIDs) await record.auth.admin.oauth.deleteClient(clientID);
	if (!companyID) return;
	const { data: members } = await record.from('member').select('user_id').eq('company_id', companyID);
	await record.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await record.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

async function answerOfRoute(reachRoute: () => Response | Promise<Response>): Promise<Response> {
	try {
		return await reachRoute();
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return Response.json(refusal.body, { status: refusal.status });
	}
}

function reachingThePlane(url: string | URL | Request, options?: RequestInit): Promise<Response> {
	const request = new Request(url instanceof Request ? url : String(url), options);
	const address = new URL(request.url);
	if (address.origin !== planeOrigin) return fetch(request);
	const event = { request, url: address, params: {}, platform: undefined };
	if (address.pathname === toolServer.pathname && request.method === 'POST') {
		return answerOfRoute(() => reachMCP(event as unknown as Parameters<typeof reachMCP>[0]));
	}
	if (request.method === 'GET') {
		return answerOfRoute(() =>
			readResourceMetadata(event as unknown as Parameters<typeof readResourceMetadata>[0])
		);
	}
	return Promise.resolve(new Response(null, { status: 405 }));
}

class RememberingClient implements OAuthClientProvider {
	authorizationAddress: URL | null = null;
	private information: OAuthClientInformationMixed | undefined;
	private heldTokens: OAuthTokens | undefined;
	private verifier = '';

	get redirectUrl(): string {
		return 'http://127.0.0.1:33418/callback';
	}

	get clientMetadata(): OAuthClientMetadata {
		return {
			client_name: 'MCP authorization test',
			redirect_uris: [this.redirectUrl],
			grant_types: ['authorization_code', 'refresh_token'],
			response_types: ['code'],
			token_endpoint_auth_method: 'none'
		};
	}

	clientInformation() {
		return this.information;
	}

	saveClientInformation(information: OAuthClientInformationMixed) {
		this.information = information;
		registeredClientIDs.push(information.client_id);
	}

	tokens() {
		return this.heldTokens;
	}

	saveTokens(tokens: OAuthTokens) {
		this.heldTokens = tokens;
	}

	redirectToAuthorization(authorizationAddress: URL) {
		this.authorizationAddress = authorizationAddress;
	}

	saveCodeVerifier(verifier: string) {
		this.verifier = verifier;
	}

	codeVerifier() {
		return this.verifier;
	}
}

async function theMemberInTheirBrowser(): Promise<SupabaseClient> {
	const browser = createClient(projectURL, publishableKey, { auth: { persistSession: false } });
	const signedIn = await browser.auth.signInWithPassword({ email: memberEmail, password: memberPassword });
	expect(signedIn.error).toBeNull();
	return browser;
}

async function consentAsTheMember(authorizationAddress: URL): Promise<string> {
	const toConsent = await fetch(authorizationAddress, { redirect: 'manual' });
	const consentPage = new URL(toConsent.headers.get('location') ?? '');
	expect(consentPage.pathname).toBe('/oauth/consent');

	const browser = await theMemberInTheirBrowser();
	const authorizationID = consentPage.searchParams.get('authorization_id') ?? '';
	const asked = await browser.auth.oauth.getAuthorizationDetails(authorizationID);
	expect(asked.error).toBeNull();
	const approved = await browser.auth.oauth.approveAuthorization(authorizationID, { skipBrowserRedirect: true });
	expect(approved.error).toBeNull();
	return new URL(approved.data?.redirect_url ?? '').searchParams.get('code') ?? '';
}

describe('an MCP client with no credential', () => {
	test('is challenged toward the metadata that names where to sign in', async () => {
		const answered = await reachingThePlane(toolServer, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' },
			body: '{}'
		});

		expect(answered.status).toBe(401);
		expect(answered.headers.get('www-authenticate')).toBe(
			`Bearer resource_metadata="${planeOrigin}/.well-known/oauth-protected-resource/v1/mcp"`
		);
	});

	test('reads metadata that names the tool server and the plane\'s own authorization server', async () => {
		const answered = await reachingThePlane(`${planeOrigin}/.well-known/oauth-protected-resource/v1/mcp`);
		const metadata = (await answered.json()) as { resource: string; authorization_servers: string[] };

		expect(metadata.resource).toBe(toolServer.href);
		expect(metadata.authorization_servers).toEqual([`${projectURL}/auth/v1`]);
	});

	test('finds no metadata for a path that is not a protected resource', async () => {
		const answered = await reachingThePlane(`${planeOrigin}/.well-known/oauth-protected-resource/v1/tools`);
		expect(answered.status).toBe(404);
	});

	test('signs the member in through consent, reaches their tools, and loses them when disconnected', async () => {
		const provider = new RememberingClient();
		const unauthorized = new Client({ name: 'mcp-authorization-test', version: '1' });
		const refused = unauthorized.connect(
			new StreamableHTTPClientTransport(toolServer, { authProvider: provider, fetch: reachingThePlane })
		);
		await expect(refused).rejects.toBeInstanceOf(UnauthorizedError);
		const authorizationAddress = provider.authorizationAddress;
		if (!authorizationAddress) throw new Error('the client was never sent to sign in');

		const finishing = new StreamableHTTPClientTransport(toolServer, { authProvider: provider, fetch: reachingThePlane });
		await finishing.finishAuth(await consentAsTheMember(authorizationAddress));

		const signedIn = new Client({ name: 'mcp-authorization-test', version: '1' });
		await signedIn.connect(
			new StreamableHTTPClientTransport(toolServer, { authProvider: provider, fetch: reachingThePlane })
		);
		const { tools } = await signedIn.listTools();
		await signedIn.close();
		expect(tools.map((tool) => tool.name)).toContain('task_list');

		const browser = await theMemberInTheirBrowser();
		const disconnected = await browser.auth.oauth.revokeGrant({ clientId: provider.clientInformation()?.client_id ?? '' });
		expect(disconnected.error).toBeNull();
		const afterDisconnecting = new Client({ name: 'mcp-authorization-test', version: '1' });
		await expect(
			afterDisconnecting.connect(
				new StreamableHTTPClientTransport(toolServer, { authProvider: provider, fetch: reachingThePlane })
			)
		).rejects.toBeInstanceOf(UnauthorizedError);
	}, networkHookTimeout);
});
