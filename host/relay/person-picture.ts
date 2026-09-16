import type { ActorCredential } from './forward';

export const personPictureKind = 'person-picture';

const pictureCapability = 'person.picture';
const rememberedForMilliseconds = 60 * 60 * 1000;
const readTimeLimitMilliseconds = 3000;

export type PersonPictureSources = {
	keptAlready: (digest: string) => Promise<string | null>;
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	keep: (bytes: Uint8Array, contentType: string) => Promise<string>;
	memberIDOf: (externalID: string) => Promise<string | null>;
	credentialOf: (memberID: string) => Promise<ActorCredential | null>;
	report: (line: string) => void;
	now?: () => number;
	timeLimitMilliseconds?: number;
};

export type PictureRequest = { externalID: string; avatarURL: string; actor: ActorCredential };

export class MessengerAnswered extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
	}
}

type Remembered = { path: string; keptAt: number };

// A person's picture is the object in the company's bucket that its bytes hash
// to. The messenger names a picture by that same hash (Blossom BUD-02), so one
// the company has kept is found by its name alone, and the messenger is read
// only for bytes the company has never seen.
export class PersonPictures {
	private readonly rememberedOfExternal = new Map<string, Remembered>();

	constructor(private readonly sources: PersonPictureSources) {}

	async keptPathOf(request: PictureRequest): Promise<string> {
		const digest = digestInURL(request.avatarURL);
		if (digest) {
			const kept = await this.sources.keptAlready(digest);
			if (kept) return kept;
		}
		const answer = await this.sources.askChatd(pictureCapability, {
			actor: request.actor,
			externalID: request.externalID
		});
		if (answer.status >= 300) {
			throw new MessengerAnswered(answer.status, `${pictureCapability} answered ${answer.status}`);
		}
		const picture = pictureOf(answer.body);
		if (!picture) return '';
		return this.sources.keep(picture.bytes, picture.contentType);
	}

	async pathForNotification(externalID: string): Promise<string> {
		if (!externalID) return '';
		const now = this.sources.now?.() ?? Date.now();
		const held = this.rememberedOfExternal.get(externalID);
		if (held && now - held.keptAt < rememberedForMilliseconds) return held.path;

		try {
			const path = await withinTimeLimit(
				this.pathAsTheSender(externalID),
				this.sources.timeLimitMilliseconds ?? readTimeLimitMilliseconds
			);
			this.rememberedOfExternal.set(externalID, { path, keptAt: now });
			return path;
		} catch (thrown) {
			this.sources.report(
				`sender picture of ${externalID} not kept: ${thrown instanceof Error ? thrown.message : String(thrown)}`
			);
			return '';
		}
	}

	private async pathAsTheSender(externalID: string): Promise<string> {
		const memberID = await this.sources.memberIDOf(externalID);
		if (!memberID) return '';
		const actor = await this.sources.credentialOf(memberID);
		if (!actor) return '';
		return this.keptPathOf({ externalID, avatarURL: '', actor });
	}
}

export function digestInURL(url: string): string {
	if (!url) return '';
	let lastSegment: string;
	try {
		lastSegment = new URL(url).pathname.split('/').at(-1) ?? '';
	} catch {
		return '';
	}
	const beforeExtension = lastSegment.split('.')[0] ?? '';
	return /^[a-f0-9]{64}$/.test(beforeExtension) ? beforeExtension : '';
}

async function withinTimeLimit<Value>(work: Promise<Value>, milliseconds: number): Promise<Value> {
	let timer: ReturnType<typeof setTimeout> | undefined;
	const expired = new Promise<never>((_, reject) => {
		timer = setTimeout(() => reject(new Error(`no answer within ${milliseconds}ms`)), milliseconds);
	});
	try {
		return await Promise.race([work, expired]);
	} finally {
		clearTimeout(timer);
	}
}

export function pictureOf(body: unknown): { bytes: Uint8Array; contentType: string } | null {
	const dataURL = (body as { image?: { dataURL?: unknown } | null } | null)?.image?.dataURL;
	if (typeof dataURL !== 'string') return null;
	const parsed = /^data:([^;,]+);base64,(.+)$/.exec(dataURL);
	if (!parsed) return null;
	return { contentType: parsed[1], bytes: new Uint8Array(Buffer.from(parsed[2], 'base64')) };
}
