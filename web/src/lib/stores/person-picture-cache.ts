import { readableForSeconds } from '$lib/messenger/kept-attachment';

const storageKey = 'personPicture.signed';
const renewBeforeExpiryMilliseconds = 60 * 60 * 1000;

export type KeptPicture = { address: string; signedURL: string; expiresAt: number };

export function keptPictureOf(address: string, signedURL: string, now: number = Date.now()): KeptPicture {
	return { address, signedURL, expiresAt: now + readableForSeconds * 1000 };
}

export function readKeptPictures(now: number = Date.now()): Map<string, KeptPicture> {
	if (typeof window === 'undefined') return new Map();
	try {
		const raw = window.localStorage.getItem(storageKey);
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

export function writeKeptPictures(pictures: Map<string, KeptPicture>): void {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(storageKey, JSON.stringify([...pictures]));
	} catch {
		return;
	}
}

export function forgetKeptPictures(): void {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.removeItem(storageKey);
	} catch {
		return;
	}
}
