import type { Locale } from '$lib/i18n/locale';

export const repositoryURL = 'https://github.com/yeomyeonggeori/internkim';
export const makerURL = 'https://dawn.kim';
export const startCompanyPath = '/auth/claim?new-company=1';
export const signInPath = '/attendance/';
export const installCommands = [
	'curl -fsSL https://intern.kim/install.sh | sh -s -- host',
	'sudo internkim install ~/Downloads/internkim-host.json'
];

export function docsURL(locale: Locale, page = ''): string {
	const localePrefix = locale === 'ko' ? '/ko' : '';
	return `https://docs.intern.kim${localePrefix}${page ? `/${page}` : ''}`;
}
