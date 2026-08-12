export const mailRequesterEmailHeader = 'X-INTERNKIM-REQUESTER-EMAIL';
export const mailRequesterEmailStorageKey = 'internkim.mail.requesterEmail';

type MailActorStorage = Pick<Storage, 'getItem' | 'setItem'>;

export function normalizeMailActorEmail(actorEmail: string) {
	return actorEmail.trim().toLowerCase();
}

export function isLocalMailHostname(hostname: string) {
	return hostname === '127.0.0.1' || hostname === '::1' || hostname === 'localhost';
}

export function currentMailHostname() {
	if (typeof location === 'undefined') return '';
	return location.hostname;
}

export function storedMailActorEmail(storage: MailActorStorage | undefined = localMailStorage()) {
	if (!storage) return '';
	return storage.getItem(mailRequesterEmailStorageKey) ?? '';
}

export function rememberMailActorEmail(actorEmail: string, storage: MailActorStorage | undefined = localMailStorage()) {
	const normalizedActorEmail = normalizeMailActorEmail(actorEmail);
	if (!normalizedActorEmail || !storage) return;
	storage.setItem(mailRequesterEmailStorageKey, normalizedActorEmail);
}

export function resolveMailActorEmail(
	accountEmail: string,
	draftEmail: string,
	storage: MailActorStorage | undefined = localMailStorage(),
	devActorEmail = localDevMailActorEmail()
) {
	return normalizeMailActorEmail(storedMailActorEmail(storage) || accountEmail || draftEmail || devActorEmail);
}

export function resolveMailAccountSaveActorEmail(draftEmail: string, storage: MailActorStorage | undefined = localMailStorage()) {
	return normalizeMailActorEmail(storedMailActorEmail(storage) || draftEmail);
}

export function mailRequestHeaders(actorEmail: string, headers: Record<string, string> = {}, hostname = currentMailHostname()) {
	const normalizedActorEmail = normalizeMailActorEmail(actorEmail);
	if (!normalizedActorEmail || !isLocalMailHostname(hostname)) return headers;
	return { ...headers, [mailRequesterEmailHeader]: normalizedActorEmail };
}

export function rememberLocalMailActorEmail(actorEmail: string, hostname = currentMailHostname(), storage: MailActorStorage | undefined = localMailStorage()) {
	if (!isLocalMailHostname(hostname)) return;
	rememberMailActorEmail(actorEmail, storage);
}

function localMailStorage() {
	if (typeof localStorage === 'undefined') return undefined;
	return localStorage;
}

function localDevMailActorEmail(hostname = currentMailHostname()) {
	if (!isLocalMailHostname(hostname)) return '';
	return import.meta.env?.VITE_DEV_USER_EMAIL ?? '';
}
