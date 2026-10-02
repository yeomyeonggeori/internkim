import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { compile } from 'svelte/compiler';

const layout = readFileSync(new URL('../../../src/routes/+layout.svelte', import.meta.url), 'utf8');
const gate = readFileSync(new URL('../../../src/lib/components/web-auth-gate.svelte', import.meta.url), 'utf8');

describe('shared startup boundaries', () => {
	test('does not load cross-screen search until the palette is requested', () => {
		expect(layout).not.toMatch(/import AppCommandPalette from/);
		expect(layout).toContain('if (isCommandPaletteOpen) hasRequestedCommandPalette = true;');
		expect(layout).toMatch(/\{#if hasRequestedCommandPalette\}[\s\S]*\{#await import\('\$lib\/components\/app-command-palette.svelte'\)/);
		expect(layout).toContain('<AppCommandPalette bind:open={isCommandPaletteOpen} />');
	});

	test('keeps slash, embedded-frame, button, and pending Escape controls in the light shell', () => {
		expect(layout).toContain("isPlainShortcut(event, 'Slash')");
		expect(layout).toContain("event.data.code === 'Slash'");
		expect(layout).toContain('onSearch={openCommandPalette}');
		expect(layout).toContain('onclick={() => (isCommandPaletteOpen = true)}');
		expect(layout).toContain("if (isCommandPaletteOpen && event.key === 'Escape') isCommandPaletteOpen = false;");
	});

	test('only mounts legacy identity enrollment when the company plane is absent', () => {
		expect(layout).not.toMatch(/import BuzzIdentityGate from/);
		expect(layout).toContain('needsLegacyIdentity = !isSupabaseConfigured();');
		expect(layout).toMatch(/\{#if needsLegacyIdentity\}[\s\S]*\{#await import\('\$lib\/components\/buzz\/buzz-identity-gate.svelte'\)/);
	});

	test('keeps legacy login behind user actions without changing central or Cloudflare fallback', () => {
		expect(gate).not.toMatch(/import .* from '\$lib\/buzz-key-login'/);
		expect(gate).not.toContain('createBuzzIdentityTransport');
		expect(gate.match(/await import\('\$lib\/buzz-key-login'\)/g)?.length).toBe(2);
		expect(gate).toContain('await signInWithSupabase(normalizedEmail, password);');
		expect(gate).toContain('await signInWithPasskey();');
		expect(gate).not.toContain('claimCentralBuzzSecret');
		expect(gate).toContain('href={cloudflareLoginURL}');
		expect(gate).toContain('{#if session?.authenticated}');
	});

	test('clears the account memo for each actual return before reloading session permissions', () => {
		expect(layout).toMatch(/revalidateOnReturn\(window, document, \(\) => \{\s*forgetSignedInAccount\(\);\s*void invalidate\(webAuthSessionDependency\);/);
		expect(layout).toContain('stopRevalidatingSession();');
	});

	test('compiles both boundaries for client and SSR', () => {
		const targets: Array<'client' | 'server'> = ['client', 'server'];
		for (const source of [layout, gate]) {
			for (const generate of targets) {
				expect(() => compile(source, { generate })).not.toThrow();
			}
		}
	});
});
