import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

describe('handoff overlay', () => {
	it('does not bundle the removed browser extension', () => {
		const configuration = JSON.parse(readFileSync(resolve(process.cwd(), 'src-tauri/tauri.conf.json'), 'utf8'));
		const resources = Array.isArray(configuration?.bundle?.resources) ? configuration.bundle.resources : [];

		expect(resources).not.toContain('../browser-extension');
	});

	it('routes the browser-handoff-overlay window to its own entry component', () => {
		const source = readFileSync(resolve(process.cwd(), 'src/main.ts'), 'utf8');

		expect(source).toContain("getCurrentWindow().label === 'browser-handoff-overlay'");
		expect(source).toContain('HandoffOverlay');
	});

	it('uses the native overlay window for completion', () => {
		const source = readFileSync(resolve(process.cwd(), 'src/HandoffOverlay.svelte'), 'utf8');

		expect(source).toContain("invoke('sync_handoff_overlay'");
		expect(source).toContain('`${handoffBridgeURL}/complete`');
		expect(source).toContain('완료');
	});

	it('keeps the overlay concern out of the main window App component', () => {
		const source = readFileSync(resolve(process.cwd(), 'src/App.svelte'), 'utf8');

		expect(source).not.toContain('browser-handoff-overlay');
		expect(source).not.toContain('handoffBridgeURL');
	});
});
