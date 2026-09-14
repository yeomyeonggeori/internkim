import type { ActorCredential } from './forward';

export const senderPictureKind = 'sender-picture';

const pictureCapability = 'person.picture';
const rememberedForMilliseconds = 60 * 60 * 1000;
const readTimeLimitMilliseconds = 3000;

export type SenderPictureSources = {
	memberIDOf: (externalID: string) => Promise<string | null>;
	credentialOf: (memberID: string) => Promise<ActorCredential | null>;
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	keep: (bytes: Uint8Array, contentType: string) => Promise<string>;
	report: (line: string) => void;
	now?: () => number;
	timeLimitMilliseconds?: number;
};

type Remembered = { path: string; keptAt: number };

export class SenderPictures {
	private readonly remembered = new Map<string, Remembered>();

	constructor(private readonly sources: SenderPictureSources) {}

	async pathOf(externalID: string): Promise<string> {
		if (!externalID) return '';
		const now = this.sources.now?.() ?? Date.now();
		const held = this.remembered.get(externalID);
		if (held && now - held.keptAt < rememberedForMilliseconds) return held.path;

		try {
			const path = await withinTimeLimit(
				this.keptPictureOf(externalID),
				this.sources.timeLimitMilliseconds ?? readTimeLimitMilliseconds
			);
			this.remembered.set(externalID, { path, keptAt: now });
			return path;
		} catch (thrown) {
			this.sources.report(
				`sender picture of ${externalID} not kept: ${thrown instanceof Error ? thrown.message : String(thrown)}`
			);
			return '';
		}
	}

	private async keptPictureOf(externalID: string): Promise<string> {
		const memberID = await this.sources.memberIDOf(externalID);
		if (!memberID) return '';
		const actor = await this.sources.credentialOf(memberID);
		if (!actor) return '';

		const answer = await this.sources.askChatd(pictureCapability, { actor, externalID });
		if (answer.status >= 300) throw new Error(`${pictureCapability} answered ${answer.status}`);
		const picture = pictureOf(answer.body);
		if (!picture) return '';
		return this.sources.keep(picture.bytes, picture.contentType);
	}
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
