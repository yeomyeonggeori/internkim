import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const settingsPage = readFileSync('src/routes/settings/+page.svelte', 'utf8');

describe('the settings page', () => {
	test('keeps general sections in one stable tab while the role resolves', () => {
		expect(settingsPage.match(/\{@render generalSections\(\)\}/g)?.length).toBe(1);
		expect(settingsPage).not.toMatch(/\{#if !isLoading && isAdmin\}\s*<Tabs.Root/);
	});
	test('has one arm, so a section cannot hide on the one nobody visits', () => {
		expect(settingsPage).not.toContain('DeviceSettingsPage');
	});

	test('never tests a function for truth instead of calling it', () => {
		const conditions = settingsPage.match(/\{#if [^}]*\}/g) ?? [];
		for (const condition of conditions) {
			expect(condition).not.toMatch(/[!\s(]isSupabaseConfigured[^(]/);
		}
	});
});
