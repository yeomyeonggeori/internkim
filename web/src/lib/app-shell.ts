import { companyPathOf, companySlugOf, routePathOf } from '$lib/company-path';

const appShellSections = [
	'/settings/',
	'/messenger/',
	'/task/',
	'/flow/',
	'/memory/',
	'/calendar/',
	'/crm/',
	'/mail/',
	'/attendance/',
	'/organization/',
	'/files/',
	'/runs/',
	'/assistant/'
];

const fullScreenSections = ['/handoff/'];

const embeddedCalendar = '/calendar/embed';

function isWithin(sections: string[], routePath: string): boolean {
	return sections.some((section) => routePath === section.slice(0, -1) || routePath.startsWith(section));
}

export function usesAppShell(pathname: string): boolean {
	if (isEmbeddedCalendar(pathname)) return false;
	return isWithin(appShellSections, routePathOf(pathname));
}

export function usesWebAuthGate(pathname: string): boolean {
	return isWithin([...appShellSections, ...fullScreenSections], routePathOf(pathname));
}

export function isFullScreenApp(pathname: string): boolean {
	return isWithin(fullScreenSections, routePathOf(pathname));
}

// reroute strips the company, so the address keeps it and the route does not.
// A check written against the address stops matching the moment one is there.
export function isEmbeddedCalendar(pathname: string): boolean {
	const routePath = routePathOf(pathname);
	return routePath === embeddedCalendar || routePath.startsWith(`${embeddedCalendar}/`);
}

export function taskListPathOf(pathname: string): string {
	return companyPathOf(companySlugOf(pathname), '/runs');
}

export function taskRunDetailPathOf(pathname: string, taskRunID: string): string {
	return companyPathOf(companySlugOf(pathname), `/runs/${encodeURIComponent(taskRunID)}`);
}

export function pendingApprovalsPathOf(pathname: string): string {
	return companyPathOf(companySlugOf(pathname), '/runs/approvals');
}

export function appSectionPathOf(pathname: string): string {
	const [, section = ''] = routePathOf(pathname).split('/');
	return companyPathOf(companySlugOf(pathname), `/${section}/`);
}
