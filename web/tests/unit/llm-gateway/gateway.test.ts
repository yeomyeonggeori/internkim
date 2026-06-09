import { describe, expect, test } from 'bun:test';
import {
	type DeviceTokenRecord,
	type DeviceTokenStore,
	type UsageRecord,
	handleGatewayRequest,
	reserveDeviceToken
} from '../../../../workers/llm-gateway/src/gateway';

describe('LLM Gateway Worker', () => {
	test('proxies OpenRouter-compatible requests with upstream provider key from Worker state', async () => {
		const token = 'ik_or_tenant_token';
		const tokenHash = await sha256Hex(token);
		const store = new MemoryDeviceTokenStore({
			tokenHash,
			tenantID: 'tenant-a',
			deviceID: 'device-a',
			providerAPIKey: 'sk-or-v1-upstream-a',
			hardLimitMicrounits: 100,
			requestsPerMinute: 30
		});
		const provider = new CapturingProviderClient();

		const response = await handleGatewayRequest(
			chatRequest(token, '{"model":"google/test","messages":[{"role":"user","content":"hi"}]}'),
			{
				deviceTokenStore: store,
				providerClient: provider,
				sharedSecret: 'gateway-secret',
				now: () => 1_000_000
			}
		);

		expect(response.status).toBe(200);
		expect(provider.providerAPIKey).toBe('sk-or-v1-upstream-a');
		expect(store.usageRecords.length).toBe(1);
		expect(store.records.get(tokenHash)?.usedMicrounits).toBe(12);
	});

	test('requires shared gateway secret before tenant token lookup', async () => {
		const provider = new CapturingProviderClient();
		const response = await handleGatewayRequest(chatRequestWithoutGatewaySecret('ik_or_tenant_token', '{"model":"google/test","messages":[]}'), {
			deviceTokenStore: new MemoryDeviceTokenStore(),
			providerClient: provider,
			sharedSecret: 'gateway-secret'
		});

		expect(response.status).toBe(403);
		expect(provider.callCount).toBe(0);
	});

	test('proxies OpenRouter-compatible embedding requests through the same device token boundary', async () => {
		const token = 'ik_or_tenant_token';
		const tokenHash = await sha256Hex(token);
		const store = new MemoryDeviceTokenStore({
			tokenHash,
			tenantID: 'tenant-a',
			deviceID: 'device-a',
			providerAPIKey: 'sk-or-v1-upstream-a',
			hardLimitMicrounits: 100,
			requestsPerMinute: 30
		});
		const provider = new CapturingProviderClient();

		const response = await handleGatewayRequest(
			embeddingRequest(token, '{"model":"embeddinggemma","input":"hello"}'),
			{
				deviceTokenStore: store,
				providerClient: provider,
				sharedSecret: 'gateway-secret',
				now: () => 1_000_000
			}
		);

		expect(response.status).toBe(200);
		expect(provider.embeddingCallCount).toBe(1);
		expect(provider.providerAPIKey).toBe('sk-or-v1-upstream-a');
		expect(store.usageRecords.length).toBe(1);
		expect(store.usageRecords[0].model).toBe('embeddinggemma');
	});

	test('blocks exhausted quota before provider call', async () => {
		const token = 'ik_or_tenant_token';
		const tokenHash = await sha256Hex(token);
		const store = new MemoryDeviceTokenStore({
			tokenHash,
			tenantID: 'tenant-a',
			deviceID: 'device-a',
			providerAPIKey: 'sk-or-v1-upstream-a',
			hardLimitMicrounits: 100,
			usedMicrounits: 100
		});
		const provider = new CapturingProviderClient();

		const response = await handleGatewayRequest(chatRequest(token, '{"model":"google/test","messages":[]}'), {
			deviceTokenStore: store,
			providerClient: provider,
			sharedSecret: 'gateway-secret',
			now: () => 1_000_000
		});

		expect(response.status).toBe(402);
		expect(provider.callCount).toBe(0);
		expect(store.usageRecords.length).toBe(0);
	});

	test('blocks requests per minute before provider call', async () => {
		const token = 'ik_or_tenant_token';
		const tokenHash = await sha256Hex(token);
		const store = new MemoryDeviceTokenStore({
			tokenHash,
			tenantID: 'tenant-a',
			deviceID: 'device-a',
			providerAPIKey: 'sk-or-v1-upstream-a',
			requestsPerMinute: 1
		});
		const provider = new CapturingProviderClient();
		const environment = {
			deviceTokenStore: store,
			providerClient: provider,
			sharedSecret: 'gateway-secret',
			now: () => 1_000_000
		};

		const firstResponse = await handleGatewayRequest(chatRequest(token, '{"model":"google/test","messages":[]}'), environment);
		const secondResponse = await handleGatewayRequest(chatRequest(token, '{"model":"google/test","messages":[]}'), environment);

		expect(firstResponse.status).toBe(200);
		expect(secondResponse.status).toBe(429);
		expect(provider.callCount).toBe(1);
		expect(store.usageRecords.length).toBe(1);
	});

	test('accepts admin upsert of hashed token records only', async () => {
		const store = new MemoryDeviceTokenStore();
		const response = await handleGatewayRequest(
			new Request('https://gateway.test/admin/device-tokens', {
				method: 'PUT',
				headers: {
					Authorization: 'Bearer admin-secret',
					'X-InternKim-Gateway-Secret': 'gateway-secret',
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					tokenHash: 'hash-a',
					tenantID: 'tenant-a',
					deviceID: 'device-a',
					providerAPIKey: 'sk-or-v1-upstream-a'
				})
			}),
			{
				deviceTokenStore: store,
				providerClient: new CapturingProviderClient(),
				adminToken: 'admin-secret',
				sharedSecret: 'gateway-secret'
			}
		);

		expect(response.status).toBe(200);
		expect(store.records.get('hash-a')?.providerAPIKey).toBe('sk-or-v1-upstream-a');
	});
});

class MemoryDeviceTokenStore implements DeviceTokenStore {
	records = new Map<string, DeviceTokenRecord>();
	usageRecords: UsageRecord[] = [];

	constructor(record?: DeviceTokenRecord) {
		if (record) {
			this.records.set(record.tokenHash, record);
		}
	}

	async reserve(tokenHash: string, createdAt: number) {
		const reservation = reserveDeviceToken(this.records.get(tokenHash), createdAt);
		if (reservation.reservedToken) {
			this.records.set(tokenHash, {
				...reservation.reservedToken.record,
				recentRequestTimes: reservation.reservedToken.recentRequestTimes
			});
		}
		return reservation;
	}

	async recordUsage(tokenHash: string, usageRecord: UsageRecord) {
		this.usageRecords.push(usageRecord);
		const record = this.records.get(tokenHash);
		if (record) {
			this.records.set(tokenHash, {
				...record,
				usedMicrounits: (record.usedMicrounits || 0) + usageRecord.estimatedMicrounits
			});
		}
	}

	async upsert(record: DeviceTokenRecord) {
		this.records.set(record.tokenHash, record);
	}
}

class CapturingProviderClient {
	callCount = 0;
	embeddingCallCount = 0;
	providerAPIKey = '';

	async createChatCompletion(providerAPIKey: string) {
		this.callCount += 1;
		this.providerAPIKey = providerAPIKey;
		return new Response('{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}', {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		});
	}

	async createEmbedding(providerAPIKey: string) {
		this.embeddingCallCount += 1;
		this.providerAPIKey = providerAPIKey;
		return new Response('{"model":"embeddinggemma","data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":3,"total_tokens":3}}', {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		});
	}
}

function chatRequest(token: string, body: string) {
	return new Request('https://gateway.test/api/v1/chat/completions', {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${token}`,
			'X-InternKim-Gateway-Secret': 'gateway-secret',
			'Content-Type': 'application/json'
		},
		body
	});
}

function chatRequestWithoutGatewaySecret(token: string, body: string) {
	return new Request('https://gateway.test/api/v1/chat/completions', {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${token}`,
			'Content-Type': 'application/json'
		},
		body
	});
}

function embeddingRequest(token: string, body: string) {
	return new Request('https://gateway.test/api/v1/embeddings', {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${token}`,
			'X-InternKim-Gateway-Secret': 'gateway-secret',
			'Content-Type': 'application/json'
		},
		body
	});
}

async function sha256Hex(value: string) {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
	return [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, '0')).join('');
}
