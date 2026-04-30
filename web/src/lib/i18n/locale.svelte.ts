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
}

export function setLocale(nextLocale: Locale) {
	localeValue = nextLocale;
	if (browser) localStorage.setItem(localeStorageKey, nextLocale);
}

function localeFromBrowser(): Locale {
	return navigator.language.toLowerCase().startsWith('en') ? 'en' : 'ko';
}

function parseLocale(value: string | null): Locale | undefined {
	if (value === 'ko' || value === 'en') return value;
	return undefined;
}
