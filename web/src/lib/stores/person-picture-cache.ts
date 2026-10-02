import { readableForSeconds } from '$lib/messenger/kept-attachment';

const storageKey = 'personPicture.signed';
const renewBeforeExpiryMilliseconds = 60 * 60 * 1000;

export type KeptPicture = { address: string; signedURL: string; expiresAt: number };

export function keptPictureOf(address: string, signedURL: string, now: number = Date.now()): KeptPicture {
	return { address, signedURL, expiresAt: now + readableForSeconds * 1000 };
}

export function readKeptPictures(now: number = Date.now(), scope = 'device'): Map<string, KeptPicture> {
	if (!scope) return new Map();
	if (typeof window === 'undefined') return new Map();
	try {
		const raw = window.localStorage.getItem(`${storageKey}:${scope}`);
		if (!raw) return new Map();
		const stored: [string, KeptPicture][] = JSON.parse(raw);
		if (!Array.isArray(stored)) return new Map();
		return new Map(
			stored.filter(([, picture]) => picture.expiresAt - renewBeforeExpiryMilliseconds > now)
		);
	} catch {
		return new Map();
	}
}

export function writeKeptPictures(pictures: Map<string, KeptPicture>, scope = 'device'): void {
	if (!scope) return;
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(`${storageKey}:${scope}`, JSON.stringify([...pictures]));
	} catch {
		return;
	}
}

export function forgetKeptPictures(): void {
	if (typeof window === 'undefined') return;
	try {
		for (const key of Object.keys(window.localStorage)) {
			if (key === storageKey || key.startsWith(`${storageKey}:`)) window.localStorage.removeItem(key);
		}
		window.localStorage.removeItem(`${storageKey}:device`);
	} catch {
		return;
	}
}
