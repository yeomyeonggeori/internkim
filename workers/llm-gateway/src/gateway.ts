export type DeviceTokenRecord = {
	tokenHash: string;
	tenantID: string;
	deviceID: string;
	providerAPIKey: string;
	isRevoked?: boolean;
	hardLimitMicrounits?: number;
	minimumRequestMicrounits?: number;
	requestsPerMinute?: number;
	usedMicrounits?: number;
	recentRequestTimes?: number[];
};

export type UsageRecord = {
	tenantID: string;
	deviceID: string;
	model: string;
	promptTokens: number;
	completionTokens: number;
	totalTokens: number;
	estimatedMicrounits: number;
	createdAt: string;
};

export type ReservedDeviceToken = {
	record: DeviceTokenRecord;
	recentRequestTimes: number[];
};

export type DeviceTokenReservation = {
	reservedToken?: ReservedDeviceToken;
	failureReason?: 'unknown' | 'revoked' | 'quota_exhausted' | 'rate_limited';
};

export type DeviceTokenStore = {
	reserve(tokenHash: string, createdAt: number): Promise<DeviceTokenReservation>;
	recordUsage(tokenHash: string, usageRecord: UsageRecord): Promise<void>;
	upsert(record: DeviceTokenRecord): Promise<void>;
};

export type ProviderClient = {
	createChatCompletion(providerAPIKey: string, requestDocument: string): Promise<Response>;
};

export type GatewayEnvironment = {
	deviceTokenStore: DeviceTokenStore;
	providerClient: ProviderClient;
	adminToken?: string;
	sharedSecret?: string;
	now?: () => number;
};

export async function handleGatewayRequest(request: Request, environment: GatewayEnvironment): Promise<Response> {
	const url = new URL(request.url);
	if (request.method === 'GET' && url.pathname === '/health') {
		return jsonResponse({ status: 'ok' }, 200);
	}
	if (!hasSharedSecret(request, environment.sharedSecret)) {
		return jsonResponse({ error: 'gateway secret is required' }, 403);
	}
	if (request.method === 'PUT' && url.pathname === '/admin/device-tokens') {
		return handleDeviceTokenUpsert(request, environment);
	}
	if (request.method !== 'POST' || !chatCompletionPath(url.pathname)) {
		return jsonResponse({ error: 'not found' }, 404);
	}
	return handleChatCompletions(request, environment);
}

async function handleDeviceTokenUpsert(request: Request, environment: GatewayEnvironment): Promise<Response> {
	if (!hasAdminAuthorization(request, environment.adminToken)) {
		return jsonResponse({ error: 'admin token is required' }, 401);
	}
	const document = await request.json();
	const record = parseDeviceTokenRecord(document);
	if (!record) {
		return jsonResponse({ error: 'device token record is invalid' }, 400);
	}
	await environment.deviceTokenStore.upsert(record);
	return jsonResponse({ status: 'ok' }, 200);
}

async function handleChatCompletions(request: Request, environment: GatewayEnvironment): Promise<Response> {
	const token = bearerToken(request);
	if (!token) {
		return jsonResponse({ error: 'device token is required' }, 401);
	}
	const tokenHash = await sha256Hex(token);
	const reservation = await environment.deviceTokenStore.reserve(tokenHash, environmentNow(environment));
	if (!reservation.reservedToken) {
		return failedReservationResponse(reservation.failureReason);
	}
	const requestDocument = await request.text();
	const providerResponse = await environment.providerClient.createChatCompletion(reservation.reservedToken.record.providerAPIKey, requestDocument);
	const responseDocument = await providerResponse.text();
	if (providerResponse.status >= 400) {
		return new Response(responseDocument, responseMetadata(providerResponse.status));
	}
	await environment.deviceTokenStore.recordUsage(tokenHash, usageRecordForResponse(reservation.reservedToken.record, requestDocument, responseDocument));
	return new Response(responseDocument, responseMetadata(providerResponse.status));
}

function parseDeviceTokenRecord(document: unknown): DeviceTokenRecord | undefined {
	if (!isObject(document)) return undefined;
	if (!isString(document.tokenHash)) return undefined;
	if (!isString(document.tenantID)) return undefined;
	if (!isString(document.deviceID)) return undefined;
	if (!isString(document.providerAPIKey)) return undefined;
	return {
		tokenHash: document.tokenHash.trim(),
		tenantID: document.tenantID.trim(),
		deviceID: document.deviceID.trim(),
		providerAPIKey: document.providerAPIKey.trim(),
		isRevoked: document.isRevoked === true,
		hardLimitMicrounits: optionalNumber(document.hardLimitMicrounits),
		minimumRequestMicrounits: optionalNumber(document.minimumRequestMicrounits),
		requestsPerMinute: optionalNumber(document.requestsPerMinute),
		usedMicrounits: optionalNumber(document.usedMicrounits),
		recentRequestTimes: optionalNumberList(document.recentRequestTimes)
	};
}

export function reserveDeviceToken(record: DeviceTokenRecord | undefined, createdAt: number): DeviceTokenReservation {
	if (!record) return { failureReason: 'unknown' };
	if (record.isRevoked) return { failureReason: 'revoked' };
	if (quotaIsExhausted(record)) return { failureReason: 'quota_exhausted' };
	const recentRequestTimes = recentRequestTimesFor(record, createdAt);
	if (rateLimitIsExceeded(record, recentRequestTimes)) return { failureReason: 'rate_limited' };
	return { reservedToken: { record, recentRequestTimes: [...recentRequestTimes, createdAt] } };
}

function quotaIsExhausted(record: DeviceTokenRecord): boolean {
	const hardLimitMicrounits = record.hardLimitMicrounits || 0;
	if (hardLimitMicrounits <= 0) return false;
	const minimumRequestMicrounits = record.minimumRequestMicrounits || 1;
	return (record.usedMicrounits || 0) + minimumRequestMicrounits > hardLimitMicrounits;
}

function recentRequestTimesFor(record: DeviceTokenRecord, createdAt: number): number[] {
	const windowStart = createdAt - 60_000;
	return (record.recentRequestTimes || []).filter(requestTime => requestTime > windowStart);
}

function rateLimitIsExceeded(record: DeviceTokenRecord, recentRequestTimes: number[]): boolean {
	const requestsPerMinute = record.requestsPerMinute || 0;
	return requestsPerMinute > 0 && recentRequestTimes.length >= requestsPerMinute;
}

function usageRecordForResponse(record: DeviceTokenRecord, requestDocument: string, responseDocument: string): UsageRecord {
	const responseUsage = parseResponseUsage(responseDocument);
	const estimatedMicrounits = responseUsage.totalTokens || record.minimumRequestMicrounits || 1;
	return {
		tenantID: record.tenantID,
		deviceID: record.deviceID,
		model: parseRequestModel(requestDocument),
		promptTokens: responseUsage.promptTokens,
		completionTokens: responseUsage.completionTokens,
		totalTokens: responseUsage.totalTokens,
		estimatedMicrounits,
		createdAt: new Date().toISOString()
	};
}

function parseRequestModel(document: string): string {
	const parsedDocument = parseJSON(document);
	if (!isObject(parsedDocument) || !isString(parsedDocument.model)) return '';
	return parsedDocument.model.trim();
}

function parseResponseUsage(document: string) {
	const parsedDocument = parseJSON(document);
	if (!isObject(parsedDocument) || !isObject(parsedDocument.usage)) {
		return { promptTokens: 0, completionTokens: 0, totalTokens: 0 };
	}
	const promptTokens = optionalNumber(parsedDocument.usage.prompt_tokens) || 0;
	const completionTokens = optionalNumber(parsedDocument.usage.completion_tokens) || 0;
	const totalTokens = optionalNumber(parsedDocument.usage.total_tokens) || promptTokens + completionTokens;
	return { promptTokens, completionTokens, totalTokens };
}

function bearerToken(request: Request): string {
	const authorization = request.headers.get('Authorization') || '';
	if (!authorization.startsWith('Bearer ')) return '';
	return authorization.slice('Bearer '.length).trim();
}

function hasAdminAuthorization(request: Request, adminToken: string | undefined): boolean {
	const token = bearerToken(request);
	return Boolean(adminToken && token && token === adminToken);
}

function hasSharedSecret(request: Request, sharedSecret: string | undefined): boolean {
	if (!sharedSecret) return true;
	return request.headers.get('X-InternKim-Gateway-Secret') === sharedSecret;
}

function failedReservationResponse(failureReason: DeviceTokenReservation['failureReason']): Response {
	if (failureReason === 'quota_exhausted') {
		return jsonResponse({ error: 'tenant llm quota is exhausted' }, 402);
	}
	if (failureReason === 'rate_limited') {
		return jsonResponse({ error: 'tenant llm rate limit is exceeded' }, 429);
	}
	return jsonResponse({ error: 'device token is revoked or unknown' }, 401);
}

function chatCompletionPath(pathname: string): boolean {
	return pathname === '/api/v1/chat/completions' || pathname === '/v1/chat/completions';
}

async function sha256Hex(value: string): Promise<string> {
	const encodedValue = new TextEncoder().encode(value);
	const digest = await crypto.subtle.digest('SHA-256', encodedValue);
	return [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, '0')).join('');
}

function jsonResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), responseMetadata(status));
}

function responseMetadata(status: number): ResponseInit {
	return { status, headers: { 'Content-Type': 'application/json' } };
}

function environmentNow(environment: GatewayEnvironment): number {
	return environment.now ? environment.now() : Date.now();
}

function parseJSON(document: string): unknown {
	try {
		return JSON.parse(document);
	} catch {
		return undefined;
	}
}

function isObject(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
	return typeof value === 'string';
}

function optionalNumber(value: unknown): number | undefined {
	return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function optionalNumberList(value: unknown): number[] | undefined {
	if (!Array.isArray(value)) return undefined;
	const numbers = value.filter(item => typeof item === 'number' && Number.isFinite(item));
	return numbers.length === value.length ? numbers : undefined;
}
