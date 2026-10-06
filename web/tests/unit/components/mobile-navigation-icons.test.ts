import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

test('mobile navigation derives its icons and labels from the same app entries as desktop', () => {
	const source = readFileSync(new URL('../../../src/lib/components/app-rail.svelte', import.meta.url), 'utf8');
	expect(source).toContain('appNavigation.apps.filter');
	expect(source).not.toContain('@lucide/svelte/icons/');
	const apps = readFileSync(new URL('../../../src/lib/components/app-navigation.svelte.ts', import.meta.url), 'utf8');
	for (const [path, icon] of [['messenger', 'MessageCircleIcon'], ['attendance', 'FlameIcon'], ['task', 'SquareCheckBigIcon'], ['calendar', 'CalendarIcon'], ['crm', 'HeartHandshakeIcon']]) {
		expect(apps).toContain(`href: this.link('/${path}/'), label: text.${path}, icon: ${icon}`);
	}
});
