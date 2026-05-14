import { browser } from '$app/environment';

export type Locale = 'ko' | 'en';

const localeStorageKey = 'internkim.locale';

let localeValue = $state<Locale>('ko');

export const localeOptions: { value: Locale; label: string; shortLabel: string }[] = [
	{ value: 'ko', label: '한국어', shortLabel: 'KO' },
	{ value: 'en', label: 'English', shortLabel: 'EN' }
];

export const currentLocale = {
	get value() {
		return localeValue;
	}
};

export function initializeLocale() {
	if (!browser) return;

	const storedLocale = parseLocale(localStorage.getItem(localeStorageKey));
	localeValue = storedLocale ?? localeFromBrowser();
	void loadServerLocale();
}

export function setLocale(nextLocale: Locale) {
	localeValue = nextLocale;
	if (browser) localStorage.setItem(localeStorageKey, nextLocale);
	if (browser) void saveServerLocale(nextLocale);
}

function localeFromBrowser(): Locale {
	return navigator.language.toLowerCase().startsWith('en') ? 'en' : 'ko';
}

function parseLocale(value: string | null): Locale | undefined {
	if (value === 'ko' || value === 'en') return value;
	return undefined;
}

async function loadServerLocale() {
	try {
		const response = await fetch('/admin/api/locale', { credentials: 'include' });
		if (!response.ok) return;
		const payload = (await response.json()) as { locale?: string };
		const locale = parseLocale(payload.locale ?? null);
		if (!locale) return;
		localeValue = locale;
		localStorage.setItem(localeStorageKey, locale);
	} catch {
		return;
	}
}

async function saveServerLocale(locale: Locale) {
	try {
		await fetch('/admin/api/locale', {
			method: 'PUT',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ locale })
		});
	} catch {
		return;
	}
}
