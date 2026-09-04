import { createHash, randomUUID } from 'node:crypto';
import { mkdir, open, readdir, readFile, rename, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import type { Addressing, Requester } from './acp-session';

export type HeldQuestion = {
	toolCallID: string;
	sessionID: string;
	requester: Requester;
	addressing: Addressing;
	question: string;
	askedAt: string;
	/** The person's raw words, once they arrive. */
	answer?: string;
};

export type HeldQuestionStoreSettings = {
	directoryPath: string;
	report?: (line: string) => void;
};

const questionSuffix = '.json';

export class HeldQuestionStore {
	private readonly settings: HeldQuestionStoreSettings;
	private directoryMade: Promise<void> | null = null;

	constructor(settings: HeldQuestionStoreSettings) {
		this.settings = settings;
	}

	/** Records that the question was asked. Does nothing when it is already known. */
	async keep(question: Omit<HeldQuestion, 'answer'>): Promise<void> {
		await this.makeDirectory();
		if (await this.read(question.toolCallID)) return;
		await this.write(this.pathFor(question.toolCallID), question);
	}

	/** Records the person's raw words against a question already asked. */
	async answer(toolCallID: string, words: string): Promise<void> {
		const path = this.pathFor(toolCallID);
		const held = await this.readFrom(path);
		if (!held) return;
		await this.write(path, { ...held, answer: words });
	}

	async read(toolCallID: string): Promise<HeldQuestion | null> {
		return this.readFrom(this.pathFor(toolCallID));
	}

	/** Removes the question; its answer has reached blueclaw. */
	async forget(toolCallID: string): Promise<void> {
		await unlink(this.pathFor(toolCallID)).catch(missingAsNothing);
	}

	/** Every question still open across a restart, asked and unanswered alike. */
	async all(): Promise<HeldQuestion[]> {
		await this.makeDirectory();
		const names = await readdir(this.settings.directoryPath);
		const questions: HeldQuestion[] = [];
		for (const name of names) {
			if (!name.endsWith(questionSuffix)) continue;
			const held = await this.readFrom(join(this.settings.directoryPath, name));
			if (held) questions.push(held);
		}
		return questions;
	}

	private makeDirectory(): Promise<void> {
		this.directoryMade ??= mkdir(this.settings.directoryPath, { recursive: true }).then(() => undefined);
		return this.directoryMade;
	}

	private pathFor(toolCallID: string): string {
		const named = createHash('sha256').update(toolCallID).digest('hex');
		return join(this.settings.directoryPath, `${named}${questionSuffix}`);
	}

	private async readFrom(path: string): Promise<HeldQuestion | null> {
		const written = await readFile(path, 'utf8').catch(missingAsNothing);
		if (written === null) return null;
		const held = readHeldQuestion(parsedOrNothing(written));
		if (!held) {
			this.settings.report?.(`a held question will not parse and was left in place: ${path}`);
			return null;
		}
		return held;
	}

	private async write(
		path: string,
		question: HeldQuestion | Omit<HeldQuestion, 'answer'>
	): Promise<void> {
		const writingPath = `${path}.${randomUUID()}.writing`;
		const file = await open(writingPath, 'w');
		try {
			await file.writeFile(JSON.stringify(question));
			await file.sync();
		} finally {
			await file.close();
		}
		// POSIX.1-2017 rename() replaces the name in one step, so an interrupted
		// write leaves either the whole question under its own name or no file at all.
		await rename(writingPath, path);
	}
}

function readHeldQuestion(offered: unknown): HeldQuestion | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;

	const toolCallID = held.toolCallID;
	const sessionID = held.sessionID;
	const question = held.question;
	const askedAt = held.askedAt;
	const requester = held.requester;
	const addressing = held.addressing;
	const answer = held.answer;
	if (typeof toolCallID !== 'string' || toolCallID === '') return null;
	if (typeof sessionID !== 'string' || sessionID === '') return null;
	if (typeof question !== 'string') return null;
	if (typeof askedAt !== 'string' || askedAt === '') return null;
	if (typeof requester !== 'object' || requester === null) return null;
	if (typeof addressing !== 'object' || addressing === null) return null;

	return {
		toolCallID,
		sessionID,
		question,
		askedAt,
		requester: requester as Requester,
		addressing: addressing as Addressing,
		...(typeof answer === 'string' ? { answer } : {})
	};
}

function parsedOrNothing(written: string): unknown {
	try {
		return JSON.parse(written);
	} catch {
		return null;
	}
}

async function missingAsNothing(failure: unknown): Promise<null> {
	if (isMissing(failure)) return null;
	throw failure;
}

function isMissing(failure: unknown): boolean {
	if (typeof failure !== 'object' || failure === null || !('code' in failure)) return false;
	return failure.code === 'ENOENT';
}
