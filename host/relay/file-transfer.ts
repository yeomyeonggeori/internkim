import { attachmentAddress, attachmentKind, sharedAssetPath } from './asset-store';
import {
	copyIntoStore,
	copyWithinStore,
	keptSizeOf,
	removeFromStore,
	signedReadURL,
	storeRangeReader,
	storeRangesAsStream,
	TransferFailed,
	type CopyProgress,
	type RangedSource,
	type StoreAccess
} from './transfer-store';
import type { Transfers, TransferWatcher } from './transfers';

export type ActorCredential = { kind: string; secret: string };

export type Answered = { status: number; body: unknown };

export type FileTransferDependencies = {
	companyID: string;
	hostAccess: StoreAccess;
	memberAccess: (requesterEmail: string) => StoreAccess;
	transfers: Transfers;
	readMediaRange: (actor: ActorCredential, mediaURL: string, rangeHeader: string) => Promise<Response>;
	uploadMedia: (actor: ActorCredential, sourceURL: string, contentType: string) => Promise<Answered>;
	listWorkspaceDirectory: (requesterEmail: string, directoryPath: string) => Promise<Answered>;
	readWorkspaceRange: (requesterEmail: string, path: string, rangeHeader: string) => Promise<Response>;
	writeWorkspaceFile: (
		write: { requester: string; permission?: string; path: string },
		body: ReadableStream<Uint8Array>
	) => Promise<Answered>;
	report: (line: string) => void;
};

export type WorkspaceFile = {
	filename: string;
	workspacePath: string;
	contentType: string;
};

export type KeptAttachment = {
	filename: string;
	contentType: string;
	address: string;
	digest: string;
	sizeBytes: number;
};

const transferKind = 'transfer';
const uploadReadableSeconds = 60 * 60;
const transferIDPattern = /^[A-Za-z0-9-]{8,64}$/;
const digestPattern = /^[0-9a-f]{64}$/;
const defaultContentType = 'application/octet-stream';
const refusedByTheMessengerStatus = 415;

function text(body: Record<string, unknown>, field: string): string {
	const value = body[field];
	return typeof value === 'string' ? value.trim() : '';
}

function refused(status: number, error: string): Answered {
	return { status, body: { error } };
}

function pending(): Answered {
	return { status: 200, body: { transfer: { state: 'pending' } } };
}

function ready(projectURL: string, path: string, sizeBytes: number): Answered {
	return { status: 200, body: { transfer: { state: 'ready', address: attachmentAddress(projectURL, path), sizeBytes } } };
}

function digestOfText(value: string): string {
	return new Bun.CryptoHasher('sha256').update(value).digest('hex');
}

function reasonOf(body: unknown, status: number): string {
	const said = (body as { error?: unknown } | null)?.error;
	return typeof said === 'string' && said.trim() !== '' ? said : `answered ${status}`;
}

function watcherOf(memberID: string, body: Record<string, unknown>): TransferWatcher | null {
	const transferID = text(body, 'transferID');
	return transferIDPattern.test(transferID) ? { memberID, transferID } : null;
}

async function preparedCopy(
	dependencies: FileTransferDependencies,
	path: string,
	watcher: TransferWatcher,
	copy: (progress: CopyProgress) => Promise<{ sizeBytes: number }>
): Promise<Answered> {
	const { projectURL } = dependencies.hostAccess;
	if (!dependencies.transfers.isRunning(path)) {
		const keptBytes = await keptSizeOf(dependencies.hostAccess, path);
		if (keptBytes !== null) return ready(projectURL, path, keptBytes);
	}
	dependencies.transfers.start(path, watcher, async (progress) => {
		const copied = await copy(progress);
		return { address: attachmentAddress(projectURL, path), sizeBytes: copied.sizeBytes };
	});
	return pending();
}

export async function prepareMedia(
	dependencies: FileTransferDependencies,
	memberID: string,
	actor: ActorCredential,
	body: Record<string, unknown>
): Promise<Answered> {
	const watcher = watcherOf(memberID, body);
	const mediaURL = text(body, 'mediaURL');
	if (!watcher || !mediaURL) return refused(400, 'a media transfer names a transferID and the mediaURL it reads');
	const claimedDigest = text(body, 'digest').toLowerCase();
	const digest = digestPattern.test(claimedDigest) ? claimedDigest : '';
	const contentType = text(body, 'contentType') || defaultContentType;
	const key = digest || digestOfText(`media\n${mediaURL}`);
	const path = sharedAssetPath(dependencies.companyID, transferKind, key, contentType);
	const source: RangedSource = (rangeHeader) => dependencies.readMediaRange(actor, mediaURL, rangeHeader);
	return preparedCopy(dependencies, path, watcher, (progress) =>
		copyIntoStore(dependencies.hostAccess, path, contentType, source, { onProgress: progress, expectedDigest: digest })
	);
}

type WorkspaceEntry = { name: string; isDirectory: boolean; size: number; modifiedAt: string };

async function workspaceEntryOf(
	dependencies: FileTransferDependencies,
	requesterEmail: string,
	path: string
): Promise<{ entry: WorkspaceEntry } | { refusal: Answered }> {
	const slash = path.lastIndexOf('/');
	const directoryPath = path.slice(0, slash);
	const name = path.slice(slash + 1);
	if (slash <= 0 || !name) return { refusal: refused(400, 'a workspace file is named by its absolute path') };
	const listed = await dependencies.listWorkspaceDirectory(requesterEmail, directoryPath);
	if (listed.status !== 200) return { refusal: refused(listed.status, reasonOf(listed.body, listed.status)) };
	const entries = (listed.body as { entries?: unknown } | null)?.entries;
	const entry = (Array.isArray(entries) ? entries : []).find((one) => (one as WorkspaceEntry).name === name) as
		| WorkspaceEntry
		| undefined;
	if (!entry || entry.isDirectory) return { refusal: refused(404, `there is no file called ${name} in ${directoryPath}`) };
	return { entry };
}

export async function prepareWorkspaceFile(
	dependencies: FileTransferDependencies,
	memberID: string,
	requesterEmail: string,
	body: Record<string, unknown>
): Promise<Answered> {
	const watcher = watcherOf(memberID, body);
	const filePath = text(body, 'path');
	if (!watcher || !filePath.startsWith('/')) return refused(400, 'a file transfer names a transferID and the path it reads');
	const found = await workspaceEntryOf(dependencies, requesterEmail, filePath);
	if ('refusal' in found) return found.refusal;
	const key = digestOfText(`${filePath}\n${found.entry.size}\n${found.entry.modifiedAt}`);
	const path = `${dependencies.companyID}/person/${memberID}/${transferKind}/${key}`;
	const source: RangedSource = (rangeHeader) => dependencies.readWorkspaceRange(requesterEmail, filePath, rangeHeader);
	return preparedCopy(dependencies, path, watcher, (progress) =>
		copyIntoStore(dependencies.hostAccess, path, defaultContentType, source, { onProgress: progress })
	);
}

async function letGoOfUpload(dependencies: FileTransferDependencies, object: string): Promise<void> {
	await removeFromStore(dependencies.hostAccess, [object]).catch((failure: unknown) => {
		dependencies.report(`the upload ${object} stays until its copies expire: ${failure instanceof Error ? failure.message : failure}`);
	});
}

function startUpload(
	dependencies: FileTransferDependencies,
	watcher: TransferWatcher,
	object: string,
	work: (progress: CopyProgress) => Promise<unknown>
): Answered {
	dependencies.transfers.start(`upload\n${watcher.transferID}`, watcher, async (progress) => {
		try {
			return await work(progress);
		} finally {
			await letGoOfUpload(dependencies, object);
		}
	});
	return pending();
}

async function keptInTheMessenger(
	dependencies: FileTransferDependencies,
	actor: ActorCredential,
	sourceURL: string,
	upload: { object: string; filename: string; contentType: string }
): Promise<KeptAttachment> {
	const answer = await dependencies.uploadMedia(actor, sourceURL, upload.contentType);
	const named = { filename: upload.filename, contentType: upload.contentType };
	if (answer.status === 200) {
		const media = (answer.body as { media?: { address: string; digest: string; sizeBytes: number } } | null)?.media;
		if (!media) throw new TransferFailed(502, 'the messenger kept the file and said nothing about where');
		return { ...named, address: media.address, digest: media.digest, sizeBytes: media.sizeBytes };
	}
	if (answer.status !== refusedByTheMessengerStatus) throw new TransferFailed(answer.status, reasonOf(answer.body, answer.status));
	return keptInTheCompanyBucket(dependencies, upload, answer.body);
}

async function keptInTheCompanyBucket(
	dependencies: FileTransferDependencies,
	upload: { object: string; filename: string; contentType: string },
	body: unknown
): Promise<KeptAttachment> {
	const refusal = (body as { refused?: { digest?: unknown; sizeBytes?: unknown } } | null)?.refused;
	const digest = typeof refusal?.digest === 'string' ? refusal.digest : '';
	const sizeBytes = typeof refusal?.sizeBytes === 'number' ? refusal.sizeBytes : 0;
	if (!digestPattern.test(digest)) throw new TransferFailed(502, reasonOf(body, refusedByTheMessengerStatus));
	const path = sharedAssetPath(dependencies.companyID, attachmentKind, digest, upload.contentType);
	await copyWithinStore(dependencies.hostAccess, upload.object, path);
	return {
		filename: upload.filename,
		contentType: upload.contentType,
		address: attachmentAddress(dependencies.hostAccess.projectURL, path),
		digest,
		sizeBytes
	};
}

export async function takeUploadIntoMessenger(
	dependencies: FileTransferDependencies,
	memberID: string,
	actor: ActorCredential,
	requesterEmail: string,
	body: Record<string, unknown>
): Promise<Answered> {
	const watcher = watcherOf(memberID, body);
	const upload = { object: text(body, 'object'), filename: text(body, 'filename'), contentType: text(body, 'contentType') || defaultContentType };
	if (!watcher || !upload.object || !upload.filename) {
		return refused(400, 'an upload names a transferID, the object it was put at and its filename');
	}
	let sourceURL: string;
	try {
		sourceURL = await signedReadURL(dependencies.memberAccess(requesterEmail), upload.object, uploadReadableSeconds);
	} catch (failure) {
		if (!(failure instanceof TransferFailed)) throw failure;
		return refused(failure.status, failure.message);
	}
	return startUpload(dependencies, watcher, upload.object, async () => ({
		attachment: await keptInTheMessenger(dependencies, actor, sourceURL, upload)
	}));
}

export async function keepWorkspaceFileInTheMessenger(
	dependencies: FileTransferDependencies,
	memberID: string,
	actor: ActorCredential,
	requesterEmail: string,
	file: WorkspaceFile
): Promise<KeptAttachment> {
	const object = `${dependencies.companyID}/person/${memberID}/${transferKind}/${crypto.randomUUID()}`;
	const source: RangedSource = (rangeHeader) => dependencies.readWorkspaceRange(requesterEmail, file.workspacePath, rangeHeader);
	try {
		await copyIntoStore(dependencies.hostAccess, object, file.contentType, source);
		const sourceURL = await signedReadURL(dependencies.hostAccess, object, uploadReadableSeconds);
		return await keptInTheMessenger(dependencies, actor, sourceURL, { object, filename: file.filename, contentType: file.contentType });
	} finally {
		await letGoOfUpload(dependencies, object);
	}
}

export function leafNameOf(offered: string): string {
	const named = offered.trim().replaceAll('\\', '/').split('/').pop()?.trim() ?? '';
	return ['', '.', '..', '.blueclaw'].includes(named) ? '' : named;
}

async function writtenFrom(
	dependencies: FileTransferDependencies,
	write: { requester: string; permission?: string; path: string },
	source: RangedSource,
	expectedBytes: number,
	progress?: CopyProgress
): Promise<{ path: string; sizeBytes: number }> {
	const written = await dependencies.writeWorkspaceFile(write, storeRangesAsStream(source, progress));
	if (written.status !== 200) throw new TransferFailed(written.status, reasonOf(written.body, written.status));
	const sizeBytes = (written.body as { sizeBytes?: unknown } | null)?.sizeBytes;
	if (sizeBytes !== expectedBytes) {
		throw new TransferFailed(502, `the workspace wrote ${String(sizeBytes)} bytes of a ${expectedBytes} byte file`);
	}
	return { path: write.path, sizeBytes: expectedBytes };
}

export async function takeUploadIntoWorkspace(
	dependencies: FileTransferDependencies,
	memberID: string,
	requesterEmail: string,
	body: Record<string, unknown>
): Promise<Answered> {
	const watcher = watcherOf(memberID, body);
	const object = text(body, 'object');
	const directoryPath = text(body, 'directoryPath').replace(/\/+$/, '');
	const filename = leafNameOf(text(body, 'filename'));
	if (!watcher || !object || !directoryPath.startsWith('/') || !filename) {
		return refused(400, 'an upload names a transferID, the object it was put at, a directory and a filename');
	}
	const memberAccess = dependencies.memberAccess(requesterEmail);
	const sizeBytes = await keptSizeOf(memberAccess, object);
	if (sizeBytes === null) return refused(404, `nothing this member may read was uploaded at ${object}`);
	const write = { requester: requesterEmail, path: `${directoryPath}/${filename}` };
	return startUpload(dependencies, watcher, object, async (progress) => ({
		file: await writtenFrom(dependencies, write, storeRangeReader(memberAccess, object), sizeBytes, progress)
	}));
}

export type KeptFileReference = {
	requester: string;
	permission: string;
	digest: string;
	contentType: string;
	filename: string;
};

export async function writeKeptFileIntoWorkspace(
	dependencies: FileTransferDependencies,
	kept: KeptFileReference,
	path: string
): Promise<{ path: string; sizeBytes: number }> {
	const object = sharedAssetPath(dependencies.companyID, attachmentKind, kept.digest, kept.contentType);
	const sizeBytes = await keptSizeOf(dependencies.hostAccess, object);
	if (sizeBytes === null) throw new TransferFailed(404, `the asset store holds nothing at ${object}`);
	const write = { requester: kept.requester, permission: kept.permission, path };
	return writtenFrom(dependencies, write, storeRangeReader(dependencies.hostAccess, object), sizeBytes);
}

const removedAtOnce = 100;

export async function removeExpiredCopies(dependencies: FileTransferDependencies, paths: string[]): Promise<number> {
	for (let start = 0; start < paths.length; start += removedAtOnce) {
		await removeFromStore(dependencies.hostAccess, paths.slice(start, start + removedAtOnce));
	}
	return paths.length;
}
