import { mkdir, readFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import type { HeldSession } from './acp-session';
import { writeDurably } from './durable-write';

export type HeldSessionStoreSettings = {
	filePath: string;
	report?: (line: string) => void;
};

export class HeldSessionStore {
	private readonly settings: HeldSessionStoreSettings;
	private writing: Promise<void> = Promise.resolve();

	constructor(settings: HeldSessionStoreSettings) {
		this.settings = settings;
	}

	async all(): Promise<HeldSession[]> {
		const written = await readFile(this.settings.filePath, 'utf8').catch(missingAsNothing);
		if (written === null) return [];
		const offered = parsedOrNothing(written);
		if (Array.isArray(offered)) return offered.filter(isHeldSession);
		this.settings.report?.(`the held sessions will not parse and were left in place: ${this.settings.filePath}`);
		return [];
	}

	keep(held: HeldSession): Promise<void> {
		this.writing = this.writing.then(() => this.keepOnDisk(held));
		return this.writing;
	}

	private async keepOnDisk(held: HeldSession): Promise<void> {
		const known = (await this.all()).filter((other) => other.addressing.conversationID !== held.addressing.conversationID);
		await mkdir(dirname(this.settings.filePath), { recursive: true });
		await writeDurably(this.settings.filePath, JSON.stringify([...known, held]));
	}
}

const requesterTexts = ['name', 'callingName', 'handle'];
const addressingTexts = ['conversationType', 'replyTargetID', 'responseLanguage'];

function isHeldSession(offered: unknown): offered is HeldSession {
	if (!isRecord(offered) || !isRecord(offered.requester) || !isRecord(offered.addressing)) return false;
	const { requester, addressing } = offered;
	return (
		isFilled(offered.sessionID) &&
		isFilled(requester.email) &&
		isFilled(addressing.platform) &&
		isFilled(addressing.conversationID) &&
		[...requesterTexts.map((name) => requester[name]), ...addressingTexts.map((name) => addressing[name])].every(isTextOrAbsent) &&
		(addressing.isThread === undefined || typeof addressing.isThread === 'boolean')
	);
}

function isTextOrAbsent(offered: unknown): boolean {
	return offered === undefined || typeof offered === 'string';
}

function isFilled(offered: unknown): boolean {
	return typeof offered === 'string' && offered !== '';
}

function isRecord(offered: unknown): offered is Record<string, unknown> {
	return typeof offered === 'object' && offered !== null;
}

function parsedOrNothing(written: string): unknown {
	try {
		return JSON.parse(written);
	} catch {
		return null;
	}
}

async function missingAsNothing(failure: unknown): Promise<null> {
	if (typeof failure === 'object' && failure !== null && 'code' in failure && failure.code === 'ENOENT') return null;
	throw failure;
}
