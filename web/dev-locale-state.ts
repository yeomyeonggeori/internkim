export type DevLocale = 'ko' | 'en';

const state: { locale: DevLocale } = { locale: 'ko' };

export function devLocale(): DevLocale {
	return state.locale;
}

export function setDevLocale(locale: DevLocale): void {
	state.locale = locale;
}
