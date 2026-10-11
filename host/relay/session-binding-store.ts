import { randomUUID } from 'node:crypto';
import { mkdir, readFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import type { SessionBinding } from './acp-session';
import { writeDurably } from './durable-write';

export type SessionBindingStoreSettings = {
	filePath: string;
	report?: (line: string) => void;
};

export class SessionBindingStore {
	private readonly settings: SessionBindingStoreSettings;
	private writing: Promise<void> = Promise.resolve();

	constructor(settings: SessionBindingStoreSettings) {
		this.settings = settings;
	}

	async list(): Promise<SessionBinding[]> {
		const written = await this.readFromDisk();
		const bindings = written.map(withCatalogTicket);
		if (!written.every(hasCatalogTicket)) await this.replaceAll(bindings);
		return bindings;
	}

	save(binding: SessionBinding): Promise<void> {
		this.writing = this.writing.then(() => this.saveToDisk(binding));
		return this.writing;
	}

	private replaceAll(bindings: SessionBinding[]): Promise<void> {
		this.writing = this.writing.then(() => this.writeToDisk(bindings));
		return this.writing;
	}

	private async readFromDisk(): Promise<WrittenBinding[]> {
		const written = await readFile(this.settings.filePath, 'utf8').catch(nothingWhenMissing);
		if (written === null) return [];
		const offered = nothingWhenUnparsable(written);
		if (Array.isArray(offered)) return offered.filter(isSessionBinding);
		this.settings.report?.(`the binding sessions will not parse and were left in place: ${this.settings.filePath}`);
		return [];
	}

	private async saveToDisk(binding: SessionBinding): Promise<void> {
		const known = (await this.readFromDisk())
			.filter((other) => other.addressing.conversationID !== binding.addressing.conversationID)
			.map(withCatalogTicket);
		await this.writeToDisk([...known, binding]);
	}

	private async writeToDisk(bindings: SessionBinding[]): Promise<void> {
		await mkdir(dirname(this.settings.filePath), { recursive: true });
		await writeDurably(this.settings.filePath, JSON.stringify(bindings));
	}
}

type WrittenBinding = Omit<SessionBinding, 'catalogTicket'> & { catalogTicket?: string };

function hasCatalogTicket(binding: WrittenBinding): boolean {
	return binding.catalogTicket !== undefined;
}

function withCatalogTicket(binding: WrittenBinding): SessionBinding {
	return { ...binding, catalogTicket: binding.catalogTicket ?? randomUUID() };
}

const requesterTexts = ['name', 'callingName', 'handle'];
const addressingTexts = ['conversationType', 'replyTargetID', 'responseLanguage'];

function isSessionBinding(offered: unknown): offered is WrittenBinding {
	if (!isRecord(offered) || !isRecord(offered.requester) || !isRecord(offered.addressing)) return false;
	const { requester, addressing } = offered;
	return (
		isFilled(offered.sessionID) &&
		isFilled(requester.email) &&
		isFilled(addressing.platform) &&
		isFilled(addressing.conversationID) &&
		(offered.catalogTicket === undefined || isFilled(offered.catalogTicket)) &&
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

function nothingWhenUnparsable(written: string): unknown {
	try {
		return JSON.parse(written);
	} catch {
		return null;
	}
}

async function nothingWhenMissing(failure: unknown): Promise<null> {
	if (typeof failure === 'object' && failure !== null && 'code' in failure && failure.code === 'ENOENT') return null;
	throw failure;
}
