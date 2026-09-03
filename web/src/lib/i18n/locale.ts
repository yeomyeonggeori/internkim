export type Locale = 'ko' | 'en';

export function localeOf(tag: string | null | undefined): Locale {
	return (tag ?? '').trim().toLowerCase().startsWith('en') ? 'en' : 'ko';
}
