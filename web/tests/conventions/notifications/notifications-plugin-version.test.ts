import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const webDirectory = fileURLToPath(new URL('../../../', import.meta.url));

function pageAPIVersion(): string {
	const manifest: { dependencies: Record<string, string> } = JSON.parse(
		readFileSync(`${webDirectory}package.json`, 'utf8')
	);
	return manifest.dependencies['@choochmeque/tauri-plugin-notifications-api'] ?? '';
}

function appPluginVersion(): string {
	const cargoManifest = readFileSync(`${webDirectory}src-tauri/Cargo.toml`, 'utf8');
	return cargoManifest.match(/^tauri-plugin-notifications = \{ version = "([^"]+)"/m)?.[1] ?? '';
}

describe('the notifications plugin', () => {
	test('is the same version in the page and in the app, so the commands one sends are the ones the other answers', () => {
		expect(appPluginVersion()).not.toBe('');
		expect(pageAPIVersion()).toBe(appPluginVersion());
	});
});
