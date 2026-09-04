import { createHash, randomUUID } from 'node:crypto';
import { mkdir, open, readdir, readFile, rename, stat, unlink } from 'node:fs/promises';
import { join } from 'node:path';

export type QueuedInboundEvent = {
	key: string;
	body: unknown;
	attempts: number;
	firstQueuedAt: string;
};

export type InboundQueueSettings = {
	directoryPath: string;
	attemptCeiling?: number;
	report?: (line: string) => void;
};

const defaultAttemptCeiling = 8;
const eventSuffix = '.json';

export class InboundQueue {
	private readonly settings: InboundQueueSettings;
	private readonly attemptCeiling: number;
	private directoryMade: Promise<void> | null = null;

	constructor(settings: InboundQueueSettings) {
		this.settings = settings;
		this.attemptCeiling = settings.attemptCeiling ?? defaultAttemptCeiling;
	}

	/** Writes the event to disk. Returns false when this key is already known. */
	async keep(key: string, body: unknown): Promise<boolean> {
		await this.makeDirectory();
		const path = this.pathFor(key);
		if (await alreadyWritten(path)) return false;
		await this.write(path, { key, body, attempts: 0, firstQueuedAt: new Date().toISOString() });
		return true;
	}

	/** Every event still undelivered, oldest first. Safe to call after a restart. */
	async undelivered(): Promise<QueuedInboundEvent[]> {
		await this.makeDirectory();
		const names = await readdir(this.settings.directoryPath);
		const events: QueuedInboundEvent[] = [];
		for (const name of names) {
			if (!name.endsWith(eventSuffix)) continue;
			const event = await this.readEvent(join(this.settings.directoryPath, name));
			if (event) events.push(event);
		}
		return events.sort(oldestFirst);
	}

	/** Records one failed delivery. Returns the attempts so far. */
	async recordAttempt(key: string): Promise<number> {
		const path = this.pathFor(key);
		const event = await this.readEvent(path);
		if (!event) return 0;
		const attempted = { ...event, attempts: event.attempts + 1 };
		await this.write(path, attempted);
		return attempted.attempts;
	}

	/** Removes the event; it has been delivered, or given up on. */
	async forget(key: string): Promise<void> {
		await unlink(this.pathFor(key)).catch(missingAsNothing);
	}

	/** True when this key has been through attemptCeiling attempts. */
	hasExhausted(event: QueuedInboundEvent): boolean {
		return event.attempts >= this.attemptCeiling;
	}

	private makeDirectory(): Promise<void> {
		this.directoryMade ??= mkdir(this.settings.directoryPath, { recursive: true }).then(() => undefined);
		return this.directoryMade;
	}

	private pathFor(key: string): string {
		const named = createHash('sha256').update(key).digest('hex');
		return join(this.settings.directoryPath, `${named}${eventSuffix}`);
	}

	private async readEvent(path: string): Promise<QueuedInboundEvent | null> {
		const written = await readFile(path, 'utf8').catch(missingAsNothing);
		if (written === null) return null;
		const event = readQueuedEvent(parsedOrNothing(written));
		if (!event) {
			this.settings.report?.(`a queued inbound event will not parse and was left in place: ${path}`);
			return null;
		}
		return event;
	}

	private async write(path: string, event: QueuedInboundEvent): Promise<void> {
		const writingPath = `${path}.${randomUUID()}.writing`;
		const file = await open(writingPath, 'w');
		try {
			await file.writeFile(JSON.stringify(event));
			await file.sync();
		} finally {
			await file.close();
		}
		// POSIX.1-2017 rename() replaces the name in one step, so an interrupted
		// write leaves either the whole event under its own name or no file at all.
		await rename(writingPath, path);
	}
}

function readQueuedEvent(offered: unknown): QueuedInboundEvent | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;

	const key = held.key;
	const attempts = held.attempts;
	const firstQueuedAt = held.firstQueuedAt;
	if (typeof key !== 'string' || key === '') return null;
	if (typeof attempts !== 'number' || !Number.isFinite(attempts)) return null;
	if (typeof firstQueuedAt !== 'string' || firstQueuedAt === '') return null;

	return { key, body: held.body, attempts, firstQueuedAt };
}

function parsedOrNothing(written: string): unknown {
	try {
		return JSON.parse(written);
	} catch {
		return null;
	}
}

function oldestFirst(one: QueuedInboundEvent, other: QueuedInboundEvent): number {
	if (one.firstQueuedAt !== other.firstQueuedAt) {
		return one.firstQueuedAt < other.firstQueuedAt ? -1 : 1;
	}
	if (one.key === other.key) return 0;
	return one.key < other.key ? -1 : 1;
}

async function alreadyWritten(path: string): Promise<boolean> {
	const found = await stat(path).catch(missingAsNothing);
	return found !== null;
}

async function missingAsNothing(failure: unknown): Promise<null> {
	if (isMissing(failure)) return null;
	throw failure;
}

function isMissing(failure: unknown): boolean {
	if (typeof failure !== 'object' || failure === null || !('code' in failure)) return false;
	return failure.code === 'ENOENT';
}
