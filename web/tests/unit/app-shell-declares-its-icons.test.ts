import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const webRoot = join(import.meta.dir, '..', '..');
const shell = readFileSync(join(webRoot, 'src', 'app.html'), 'utf8');
const rootLayout = readFileSync(join(webRoot, 'src', 'routes', '+layout.svelte'), 'utf8');

describe('the served HTML shell declares every icon a browser reads before hydration', () => {
	test('the shell links the tab favicon, the touch icon and the manifest', () => {
		expect(shell).toContain('<link rel="icon" type="image/png" sizes="32x32" href="/favicon.png" />');
		expect(shell).toContain('<link rel="apple-touch-icon" href="/apple-touch-icon.png" />');
		expect(shell).toContain('<link id="app-manifest" rel="manifest" href="/manifest.webmanifest" />');
	});

	test('no route declares an icon that only appears once the app has hydrated', () => {
		expect(rootLayout).not.toContain('rel="icon"');
	});
});
