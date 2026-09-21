import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const rendererKeys: string[] = (
	await import('../../../node_modules/@humanspeak/svelte-markdown/dist/utils/rendererKeys.js')
).rendererKeysInternal;

const messageBody = readFileSync(
	new URL('../../../src/lib/components/channel/channel-message-body.svelte', import.meta.url),
	'utf8'
);

const mentionText = readFileSync(
	new URL('../../../src/lib/components/channel/channel-mention-text.svelte', import.meta.url),
	'utf8'
);

describe('the message body hands the markdown parser renderers it accepts', () => {
	for (const name of ['rawtext', 'code']) {
		test(`the parser knows a "${name}" renderer`, () => {
			expect(rendererKeys).toContain(name);
		});
	}

	test('every renderer the body overrides is one the parser asks for', () => {
		const overridden = [...messageBody.matchAll(/renderers=\{\{([^}]*)\}\}/g)]
			.flatMap((match) => match[1].split(','))
			.map((entry) => entry.split(':')[0].trim())
			.filter((name) => name !== '');
		expect(overridden.length).toBeGreaterThan(0);
		for (const name of overridden) expect(rendererKeys).toContain(name);
	});

	test('the leaf renderer reads the prop the parser hands it', () => {
		expect(mentionText).toContain('text = ');
	});
});
