export const reservedFirstSegments = [
	'admin',
	'api',
	'assistant',
	'attendance',
	'auth',
	'calendar',
	'company',
	'files',
	'flow',
	'mail',
	'memory',
	'messenger',
	'ops',
	'organization',
	'poc-admin',
	'settings',
	'start',
	'tasks'
];

const reserved = new Set(reservedFirstSegments);
export const slugShape = /^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/;

export function companySlugOf(pathname: string): string {
	const [, first = ''] = pathname.split('/');
	if (reserved.has(first)) return '';
	return slugShape.test(first) ? first : '';
}

export function routePathOf(pathname: string): string {
	const slug = companySlugOf(pathname);
	if (!slug) return pathname;
	const remainder = pathname.slice(`/${slug}`.length);
	return remainder === '' ? '/' : remainder;
}

const segmentsOutsideACompany = new Set(['api', 'auth', 'start']);

export function wantsCompanyPrefix(pathname: string): boolean {
	if (companySlugOf(pathname) !== '') return false;
	const [, first = ''] = pathname.split('/');
	if (!reserved.has(first)) return false;
	return !segmentsOutsideACompany.has(first);
}

export function companyPathOf(slug: string, path: string): string {
	if (!slug) return path;
	const withinCompany = path.startsWith('/') ? path : `/${path}`;
	return withinCompany === '/' ? `/${slug}` : `/${slug}${withinCompany}`;
}
