import { readableForSeconds } from '$lib/messenger/kept-attachment';
import type { MessengerDirectory } from '$lib/messenger/messenger-directory';

const storageKey = 'personPicture.signed';
const accountsStorageKey = 'personPicture.accounts';
const renewBeforeExpiryMilliseconds = 60 * 60 * 1000;

export type KeptPicture = { address: string; signedURL: string; avatarURL: string; expiresAt: number };

export type KeptAccounts = Pick<MessengerDirectory, 'memberOfEmail' | 'externalsOfMember'>;

export function keptPictureOf(
	address: string,
	signedURL: string,
	avatarURL: string,
	now: number = Date.now()
): KeptPicture {
	return { address, signedURL, avatarURL, expiresAt: now + readableForSeconds * 1000 };
}

export function freshSignedURLOf(picture: KeptPicture, now: number = Date.now()): string {
	return picture.expiresAt - renewBeforeExpiryMilliseconds > now ? picture.signedURL : '';
}

export function readKeptPictures(scope = 'device'): Map<string, KeptPicture> {
	const stored = readStored(`${storageKey}:${scope}`, scope);
	return new Map<string, KeptPicture>(Array.isArray(stored) ? stored : []);
}

export function writeKeptPictures(pictures: Map<string, KeptPicture>, scope = 'device'): void {
	writeStored(`${storageKey}:${scope}`, scope, [...pictures]);
}

export function readKeptAccounts(scope = 'device'): KeptAccounts | null {
	const stored = readStored(`${accountsStorageKey}:${scope}`, scope);
	if (typeof stored !== 'object' || stored === null) return null;
	if (!('memberOfEmail' in stored) || !Array.isArray(stored.memberOfEmail)) return null;
	if (!('externalsOfMember' in stored) || !Array.isArray(stored.externalsOfMember)) return null;
	return {
		memberOfEmail: new Map<string, string>(stored.memberOfEmail),
		externalsOfMember: new Map<string, string[]>(stored.externalsOfMember)
	};
}

export function writeKeptAccounts(accounts: KeptAccounts, scope = 'device'): void {
	writeStored(`${accountsStorageKey}:${scope}`, scope, {
		memberOfEmail: [...accounts.memberOfEmail],
		externalsOfMember: [...accounts.externalsOfMember]
	});
}

export function forgetKeptPictures(): void {
	if (typeof window === 'undefined') return;
	try {
		for (const key of Object.keys(window.localStorage)) {
			if (isKeptPictureKey(key)) window.localStorage.removeItem(key);
		}
		window.localStorage.removeItem(`${storageKey}:device`);
		window.localStorage.removeItem(`${accountsStorageKey}:device`);
	} catch {
		return;
	}
}

function isKeptPictureKey(key: string): boolean {
	return [storageKey, accountsStorageKey].some((prefix) => key === prefix || key.startsWith(`${prefix}:`));
}

function readStored(key: string, scope: string): unknown {
	if (!scope) return null;
	if (typeof window === 'undefined') return null;
	try {
		const raw = window.localStorage.getItem(key);
		const parsed: unknown = raw ? JSON.parse(raw) : null;
		return parsed;
	} catch {
		return null;
	}
}

function writeStored(key: string, scope: string, value: unknown): void {
	if (!scope) return;
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(key, JSON.stringify(value));
	} catch {
		return;
	}
}
