import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const extensionDirectory = resolve(process.cwd(), 'browser-extension');

describe('browser extension', () => {
	it('declares the local handoff bridge permission', () => {
		const manifest = JSON.parse(readFileSync(resolve(extensionDirectory, 'manifest.json'), 'utf8')) as {
			host_permissions?: string[];
			content_scripts?: Array<{ js?: string[] }>;
		};

		expect(manifest.host_permissions).toContain('http://127.0.0.1:7983/*');
		expect(manifest.content_scripts?.[0]?.js).toContain('content-script.js');
	});

	it('renders a shadow dom overlay with one completion button', () => {
		const source = readFileSync(resolve(extensionDirectory, 'content-script.js'), 'utf8');

		expect(source).toContain("attachShadow({ mode: 'open' })");
		expect(source).toContain('완료');
		expect(source).toContain('border-radius: 6px');
		expect(source).toContain('internkim.handoff.complete');
	});
});
