import { describe, expect, test } from 'bun:test';
import { appSectionPathOf, usesAppShell, usesWebAuthGate } from '../../src/lib/app-shell';

describe('the app shell on a company address', () => {
	test('a company-prefixed app path still wears the shell', () => {
		expect(usesAppShell('/dawnstreet/calendar')).toBe(true);
		expect(usesAppShell('/dawnstreet/files/some/folder')).toBe(true);
		expect(usesAppShell('/dawnstreet/flow/')).toBe(true);
	});

	test('an app path without a company keeps working', () => {
		expect(usesAppShell('/calendar')).toBe(true);
		expect(usesAppShell('/flow/')).toBe(true);
	});

	test('the embedded calendar wears no shell, under a company or not', () => {
		expect(usesAppShell('/calendar/embed')).toBe(false);
		expect(usesAppShell('/dawnstreet/calendar/embed')).toBe(false);
		expect(usesAppShell('/dawnstreet/calendar/embed/abc')).toBe(false);
	});

	test('pages outside the apps wear no shell', () => {
		expect(usesAppShell('/')).toBe(false);
		expect(usesAppShell('/start')).toBe(false);
		expect(usesAppShell('/auth/signin')).toBe(false);
	});
});

describe('the web auth gate on a company address', () => {
	test('a company-prefixed app path is still gated', () => {
		expect(usesWebAuthGate('/dawnstreet/calendar')).toBe(true);
		expect(usesWebAuthGate('/dawnstreet/memory/graph')).toBe(true);
	});

	test('claiming an account is not gated, under a company or not', () => {
		expect(usesWebAuthGate('/auth/claim/token')).toBe(false);
		expect(usesWebAuthGate('/dawnstreet/auth/claim/token')).toBe(false);
	});
});

describe('the breadcrumb link back to the section', () => {
	test('keeps the company it was read at', () => {
		expect(appSectionPathOf('/dawnstreet/files/some/folder')).toBe('/dawnstreet/files/');
	});

	test('stays company-less when the address is', () => {
		expect(appSectionPathOf('/files/some/folder')).toBe('/files/');
	});
});
