import {
	type DeviceTokenRecord,
	type DeviceTokenReservation,
	type DeviceTokenStore,
	type UsageRecord,
	handleGatewayRequest,
	reserveDeviceToken
} from './gateway';

type WorkerEnvironment = {
	TENANT_DEVICE_TOKENS: DurableObjectNamespace;
	GATEWAY_ADMIN_TOKEN?: string;
	GATEWAY_SHARED_SECRET?: string;
	OPENROUTER_BASE_URL?: string;
};

export default {
	fetch(request: Request, environment: WorkerEnvironment) {
		return handleGatewayRequest(request, {
			deviceTokenStore: new DurableObjectDeviceTokenStore(environment.TENANT_DEVICE_TOKENS),
			providerClient: new FetchProviderClient(environment.OPENROUTER_BASE_URL),
			adminToken: environment.GATEWAY_ADMIN_TOKEN,
			sharedSecret: environment.GATEWAY_SHARED_SECRET
		});
	}
} satisfies ExportedHandler<WorkerEnvironment>;

export class TenantDeviceTokenObject {
	constructor(private readonly state: DurableObjectState) {}

	async fetch(request: Request): Promise<Response> {
		const url = new URL(request.url);
		if (request.method === 'PUT' && url.pathname === '/record') {
			return this.handleRecordUpsert(request);
		}
		if (request.method === 'POST' && url.pathname === '/reserve') {
			return this.handleReserve(request);
		}
		if (request.method === 'POST' && url.pathname === '/usage') {
			return this.handleUsage(request);
		}
		return jsonResponse({ error: 'not found' }, 404);
	}

	private async handleRecordUpsert(request: Request): Promise<Response> {
		const record = await request.json<DeviceTokenRecord>();
		await this.state.storage.put('record', record);
		return jsonResponse({ status: 'ok' }, 200);
	}

	private async handleReserve(request: Request): Promise<Response> {
		const body = await request.json<{ createdAt: number }>();
		const record = await this.state.storage.get<DeviceTokenRecord>('record');
		const reservation = reserveDeviceToken(record, body.createdAt);
		if (!reservation.reservedToken) {
			return jsonResponse(reservation, 401);
		}
		await this.state.storage.put('record', { ...reservation.reservedToken.record, recentRequestTimes: reservation.reservedToken.recentRequestTimes });
		return jsonResponse(reservation, 200);
	}

	private async handleUsage(request: Request): Promise<Response> {
		const usageRecord = await request.json<UsageRecord>();
		const record = await this.state.storage.get<DeviceTokenRecord>('record');
		if (!record) {
			return jsonResponse({ error: 'device token record is missing' }, 404);
		}
		const usedMicrounits = (record.usedMicrounits || 0) + usageRecord.estimatedMicrounits;
		await this.state.storage.put('record', { ...record, usedMicrounits });
		await this.state.storage.put(`usage:${usageRecord.createdAt}:${crypto.randomUUID()}`, usageRecord);
		return jsonResponse({ status: 'ok' }, 200);
	}
}

class DurableObjectDeviceTokenStore implements DeviceTokenStore {
	constructor(private readonly namespace: DurableObjectNamespace) {}

	async reserve(tokenHash: string, createdAt: number): Promise<DeviceTokenReservation> {
		const response = await this.stub(tokenHash).fetch('https://tenant-device-token/reserve', {
			method: 'POST',
			body: JSON.stringify({ createdAt }),
			headers: { 'Content-Type': 'application/json' }
		});
		return response.json<DeviceTokenReservation>();
	}

	async recordUsage(tokenHash: string, usageRecord: UsageRecord): Promise<void> {
		await this.stub(tokenHash).fetch('https://tenant-device-token/usage', {
			method: 'POST',
			body: JSON.stringify(usageRecord),
			headers: { 'Content-Type': 'application/json' }
		});
	}

	async upsert(record: DeviceTokenRecord): Promise<void> {
		await this.stub(record.tokenHash).fetch('https://tenant-device-token/record', {
			method: 'PUT',
			body: JSON.stringify(record),
			headers: { 'Content-Type': 'application/json' }
		});
	}

	private stub(tokenHash: string): DurableObjectStub {
		return this.namespace.get(this.namespace.idFromName(tokenHash));
	}
}

class FetchProviderClient {
	constructor(private readonly openRouterBaseURL = 'https://openrouter.ai/api/v1/chat/completions') {}

	createChatCompletion(providerAPIKey: string, requestDocument: string): Promise<Response> {
		return fetch(this.openRouterBaseURL, {
			method: 'POST',
			body: requestDocument,
			headers: {
				Authorization: `Bearer ${providerAPIKey}`,
				'Content-Type': 'application/json'
			}
		});
	}
}

function jsonResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}
