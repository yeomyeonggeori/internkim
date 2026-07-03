import { describe, expect, test } from 'bun:test';

import { appShellText } from '../../../src/lib/i18n/app-shell-text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('appShellText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(appShellText.en).sort()).toEqual(collectTextShape(appShellText.ko).sort());
	});

	test('lists every web-gated app in the sign-in description', () => {
		const protectedLabels = {
			ko: [
				appShellText.ko.flow,
				appShellText.ko.calendar,
				appShellText.ko.memory,
				appShellText.ko.mail,
				appShellText.ko.attendance,
				appShellText.ko.orgchart,
				appShellText.ko.files
			],
			en: [
				appShellText.en.flow,
				appShellText.en.calendar,
				appShellText.en.memory,
				appShellText.en.mail,
				appShellText.en.attendance,
				appShellText.en.orgchart,
				appShellText.en.files
			]
		};

		for (const label of protectedLabels.ko) {
			expect(appShellText.ko.signInDescription.includes(label)).toBe(true);
		}
		for (const label of protectedLabels.en) {
			expect(appShellText.en.signInDescription.includes(label)).toBe(true);
		}
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
