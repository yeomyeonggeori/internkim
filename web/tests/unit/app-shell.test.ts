import { describe, expect, test } from 'bun:test';
import { appSectionPathOf, isEmbeddedCalendar, taskListPathOf, usesAppShell, usesWebAuthGate } from '../../src/lib/app-shell';

describe('the app shell on a company address', () => {
	test('a company-prefixed app path still wears the shell', () => {
		expect(usesAppShell('/samplecompany/calendar')).toBe(true);
		expect(usesAppShell('/samplecompany/files/some/folder')).toBe(true);
		expect(usesAppShell('/samplecompany/flow/')).toBe(true);
		expect(usesAppShell('/samplecompany/crm')).toBe(true);
	});

	test('an app path without a company keeps working', () => {
		expect(usesAppShell('/calendar')).toBe(true);
		expect(usesAppShell('/flow/')).toBe(true);
		expect(usesAppShell('/crm')).toBe(true);
	});

	test('the embedded calendar wears no shell, under a company or not', () => {
		expect(usesAppShell('/calendar/embed')).toBe(false);
		expect(usesAppShell('/samplecompany/calendar/embed')).toBe(false);
		expect(usesAppShell('/samplecompany/calendar/embed/abc')).toBe(false);
	});

	test('pages outside the apps wear no shell', () => {
		expect(usesAppShell('/')).toBe(false);
		expect(usesAppShell('/start')).toBe(false);
		expect(usesAppShell('/auth/signin')).toBe(false);
	});
});

describe('the web auth gate on a company address', () => {
	test('a company-prefixed app path is still gated', () => {
		expect(usesWebAuthGate('/samplecompany/calendar')).toBe(true);
		expect(usesWebAuthGate('/samplecompany/memory/graph')).toBe(true);
		expect(usesWebAuthGate('/samplecompany/crm')).toBe(true);
	});

	test('claiming an account is not gated, under a company or not', () => {
		expect(usesWebAuthGate('/auth/claim/token')).toBe(false);
		expect(usesWebAuthGate('/samplecompany/auth/claim/token')).toBe(false);
	});
});

describe('the breadcrumb link back to the section', () => {
	test('keeps the company it was read at', () => {
		expect(appSectionPathOf('/samplecompany/files/some/folder')).toBe('/samplecompany/files/');
	});

	test('stays company-less when the address is', () => {
		expect(appSectionPathOf('/files/some/folder')).toBe('/files/');
	});
});

describe('a question asked of the route, not the address', () => {
	test('knows an embedded calendar whether or not a company sits in front of it', () => {
		expect(isEmbeddedCalendar('/calendar/embed')).toBe(true);
		expect(isEmbeddedCalendar('/samplecompany/calendar/embed')).toBe(true);
		expect(isEmbeddedCalendar('/samplecompany/calendar/embed/week')).toBe(true);
		expect(isEmbeddedCalendar('/samplecompany/calendar')).toBe(false);
	});

	test('sends a task back to its own list, company and all', () => {
		expect(taskListPathOf('/runs/abc')).toBe('/runs');
		expect(taskListPathOf('/samplecompany/runs/abc')).toBe('/samplecompany/runs');
	});
});
