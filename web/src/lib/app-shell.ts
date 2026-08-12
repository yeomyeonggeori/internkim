import { companyPathOf, companySlugOf, routePathOf } from '$lib/company-path';

const appShellSections = [
	'/settings/',
	'/poc-admin/',
	'/messenger/',
	'/flow/',
	'/memory/',
	'/calendar/',
	'/mail/',
	'/attendance/',
	'/organization/',
	'/files/',
	'/tasks/',
	'/assistant/'
];

const embeddedCalendar = '/calendar/embed';

function isWithin(sections: string[], routePath: string): boolean {
	return sections.some((section) => routePath === section.slice(0, -1) || routePath.startsWith(section));
}

export function usesAppShell(pathname: string): boolean {
	const routePath = routePathOf(pathname);
	if (routePath === embeddedCalendar || routePath.startsWith(`${embeddedCalendar}/`)) return false;
	return isWithin([...appShellSections, '/auth/claim/'], routePath);
}

export function usesWebAuthGate(pathname: string): boolean {
	return isWithin(appShellSections, routePathOf(pathname));
}

export function appSectionPathOf(pathname: string): string {
	const [, section = ''] = routePathOf(pathname).split('/');
	return companyPathOf(companySlugOf(pathname), `/${section}/`);
}
