import {
	messengerHTTPCapability,
	messengerMediaHeadCapability,
	messengerMediaReadCapability,
	messengerMediaStageCapability,
	messengerMediaWriteCapability
} from '../../workers/connection-gateway/src/host-protocol';
import { base64Of, bytesOfBase64 } from '../../workers/connection-gateway/src/base64';
import type { MessengerCopies } from './messenger-copies';
import { digestInURL } from './person-picture';
import { keptSizeOf, removeFromStore, storeRangeReader, storeRangesAsStream, type StoreAccess } from './transfer-store';

export const messengerCapabilities = new Set([
	messengerHTTPCapability,
	messengerMediaHeadCapability,
	messengerMediaReadCapability,
	messengerMediaStageCapability,
	messengerMediaWriteCapability
]);

export type MessengerStore = {
	companyID: string;
	access: StoreAccess;
	signedUploadURL: (path: string) => Promise<string>;
	copies: MessengerCopies;
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
const nostrAuthorizationPrefix = 'Nostr ';

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
		if (capability === messengerMediaHeadCapability) return this.headMedia(request);
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

	private async headMedia(request: RelayRequest): Promise<MessengerAnswer> {
		const response = await this.askTheRelay(request, { method: 'HEAD' });
		const answer = await answerOf(response);
		const mediaURL = `https://${request.host}${request.path}`;
		const digest = digestInURL(mediaURL);
		if (!response.ok || !digest) return answer;
		const objectURL = await this.store.copies.readableAddressOf(digest);
		if (objectURL) return { ...answer, body: { ...answer.body, objectURL } };
		const contentType = response.headers.get('content-type') ?? 'application/octet-stream';
		this.store.copies.fillAsRequester({ digest, contentType, requesterKey: requesterKeyOf(request.headers), mediaURL });
		return answer;
	}

	private async writeMedia(request: RelayRequest): Promise<MessengerAnswer> {
		if (!this.isStagingPath(request.stagedPath)) return refused(400, 'an upload names the place it was staged');
		const sizeBytes = await keptSizeOf(this.store.access, request.stagedPath);
		if (sizeBytes === null) return refused(404, 'nothing was staged for this upload');
		const hasher = new Bun.CryptoHasher('sha256');
		const staged = storeRangesAsStream(storeRangeReader(this.store.access, request.stagedPath)).pipeThrough(hashedAsItPasses(hasher));
		const headers = { ...request.headers, 'content-length': String(sizeBytes) };
		let isKeptAsCopy = false;
		try {
			const response = await this.askTheRelay({ ...request, headers }, { method: 'PUT', body: staged });
			const answer = await answerOf(response);
			const acceptedDigest = response.ok ? digestTheRelayAccepted(answer.body.bodyBase64) : '';
			if (acceptedDigest && acceptedDigest === hasher.digest('hex')) {
				isKeptAsCopy = await this.store.copies.keepUpload(request.stagedPath, acceptedDigest);
			}
			return answer;
		} finally {
			if (!isKeptAsCopy) await removeFromStore(this.store.access, [request.stagedPath]);
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

type RelayAnswer = { status: number; body: { headers: Record<string, string>; bodyBase64: string } };

async function answerOf(response: Response): Promise<RelayAnswer> {
	const body = base64Of(new Uint8Array(await response.arrayBuffer()));
	return { status: response.status, body: { headers: headersOf(response.headers), bodyBase64: body } };
}

function hashedAsItPasses(hasher: Bun.CryptoHasher): TransformStream<Uint8Array<ArrayBuffer>, Uint8Array<ArrayBuffer>> {
	return new TransformStream({
		transform(chunk, controller) {
			hasher.update(chunk);
			controller.enqueue(chunk);
		}
	});
}

function digestTheRelayAccepted(bodyBase64: string): string {
	const descriptor = parsedJSON(new TextDecoder().decode(bytesOfBase64(bodyBase64)));
	if (typeof descriptor !== 'object' || descriptor === null || !('sha256' in descriptor)) return '';
	const digest = descriptor.sha256;
	return typeof digest === 'string' && /^[0-9a-f]{64}$/.test(digest) ? digest : '';
}

function parsedJSON(text: string): unknown {
	try {
		return JSON.parse(text);
	} catch {
		return null;
	}
}

function requesterKeyOf(headers: Record<string, string>): string {
	const authorization = headers.authorization ?? '';
	if (!authorization.startsWith(nostrAuthorizationPrefix)) return '';
	const event = parsedJSON(Buffer.from(authorization.slice(nostrAuthorizationPrefix.length), 'base64').toString('utf8'));
	if (typeof event !== 'object' || event === null || !('pubkey' in event)) return '';
	const key = event.pubkey;
	return typeof key === 'string' && /^[0-9a-f]{64}$/.test(key) ? key : '';
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
