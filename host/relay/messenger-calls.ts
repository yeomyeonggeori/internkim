import {
	messengerHTTPCapability,
	messengerMediaReadCapability,
	messengerMediaStageCapability,
	messengerMediaWriteCapability
} from '../../workers/connection-gateway/src/host-protocol';
import { base64Of, bytesOfBase64 } from '../../workers/connection-gateway/src/base64';
import { keptSizeOf, removeFromStore, storeRangeReader, storeRangesAsStream, type StoreAccess } from './transfer-store';

export const messengerCapabilities = new Set([
	messengerHTTPCapability,
	messengerMediaReadCapability,
	messengerMediaStageCapability,
	messengerMediaWriteCapability
]);

export type MessengerStore = {
	companyID: string;
	access: StoreAccess;
	signedUploadURL: (path: string) => Promise<string>;
};

export type MessengerAnswer = { status: number; body: unknown };

type RelayRequest = {
	method: string;
	path: string;
	host: string;
	headers: Record<string, string>;
	bodyBase64: string;
	stagedPath: string;
};

const methodsThatCarryNoBody = new Set(['GET', 'HEAD', 'OPTIONS']);
const transferKind = 'transfer';
const stagedPrefix = 'staged-';
const uploadIDPattern = /^[0-9a-f-]{36}$/;

export class MessengerRelay {
	constructor(
		private readonly relayURL: string,
		private readonly store: MessengerStore
	) {}

	async serve(capability: string, body: Record<string, unknown>): Promise<MessengerAnswer> {
		const request = relayRequestOf(body);
		if (!request) return refused(400, `${capability} names a method, a path starting with / and the host the app dialled`);
		if (capability === messengerHTTPCapability) return this.relayHTTP(request);
		if (capability === messengerMediaReadCapability) return this.relayHTTP(request);
		if (capability === messengerMediaStageCapability) return this.stageMedia();
		if (capability === messengerMediaWriteCapability) return this.writeMedia(request);
		return refused(404, `the messenger relay has nothing called ${capability}`);
	}

	private async relayHTTP(request: RelayRequest): Promise<MessengerAnswer> {
		const body = methodsThatCarryNoBody.has(request.method) ? undefined : bytesOfBase64(request.bodyBase64);
		return answerOf(await this.askTheRelay(request, { method: request.method, body }));
	}

	private async stageMedia(): Promise<MessengerAnswer> {
		const stagedPath = `${this.stagingDirectory()}${crypto.randomUUID()}`;
		const uploadURL = await this.store.signedUploadURL(stagedPath);
		return { status: 200, body: { headers: {}, bodyBase64: '', uploadURL, stagedPath } };
	}

	private async writeMedia(request: RelayRequest): Promise<MessengerAnswer> {
		if (!this.isStagingPath(request.stagedPath)) return refused(400, 'an upload names the place it was staged');
		const sizeBytes = await keptSizeOf(this.store.access, request.stagedPath);
		if (sizeBytes === null) return refused(404, 'nothing was staged for this upload');
		const staged = storeRangesAsStream(storeRangeReader(this.store.access, request.stagedPath));
		const headers = { ...request.headers, 'content-length': String(sizeBytes) };
		try {
			return answerOf(await this.askTheRelay({ ...request, headers }, { method: 'PUT', body: staged }));
		} finally {
			await removeFromStore(this.store.access, [request.stagedPath]);
		}
	}

	private stagingDirectory(): string {
		return `${this.store.companyID}/shared/${transferKind}/${stagedPrefix}`;
	}

	private isStagingPath(path: string): boolean {
		const directory = this.stagingDirectory();
		return path.startsWith(directory) && uploadIDPattern.test(path.slice(directory.length));
	}

	private askTheRelay(request: RelayRequest, init: { method: string; body?: BodyInit }): Promise<Response> {
		return fetch(`${this.relayURL.replace(/\/+$/, '')}${request.path}`, {
			...init,
			headers: { ...request.headers, Host: request.host },
			redirect: 'manual'
		});
	}
}

function relayRequestOf(body: Record<string, unknown>): RelayRequest | null {
	const { method, path, host, headers, bodyBase64, stagedPath } = body;
	if (typeof method !== 'string' || method === '') return null;
	if (typeof path !== 'string' || !path.startsWith('/')) return null;
	if (typeof host !== 'string' || host === '') return null;
	return {
		method: method.toUpperCase(),
		path,
		host,
		headers: stringRecordOf(headers),
		bodyBase64: typeof bodyBase64 === 'string' ? bodyBase64 : '',
		stagedPath: typeof stagedPath === 'string' ? stagedPath : ''
	};
}

function stringRecordOf(value: unknown): Record<string, string> {
	if (typeof value !== 'object' || value === null) return {};
	return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
}

async function answerOf(response: Response): Promise<MessengerAnswer> {
	const body = base64Of(new Uint8Array(await response.arrayBuffer()));
	return { status: response.status, body: { headers: headersOf(response.headers), bodyBase64: body } };
}

function headersOf(headers: Headers): Record<string, string> {
	const record: Record<string, string> = {};
	headers.forEach((value, name) => {
		record[name] = value;
	});
	return record;
}

function refused(status: number, reason: string): MessengerAnswer {
	return { status, body: { error: reason } };
}
