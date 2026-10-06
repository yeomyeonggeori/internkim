import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

interface ShellOutcome {
	gateText: string | null;
	reportedErrors: string[];
	thrown: string | null;
}

async function mountShellInBrowserEnvironment(): Promise<{ contained: ShellOutcome; uncontained: ShellOutcome }> {
	const fixture = new URL('./effect-error-boundary.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn(['bun', '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errorOutput] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errorOutput).toBe(0);
	return JSON.parse(output);
}

describe('an effect that throws inside the app rail', () => {
	test('stays inside its boundary, is reported once, and lets the sign-in gate resolve', async () => {
		const { contained } = await mountShellInBrowserEnvironment();
		expect(contained.gateText).toBe('signed-in-check-resolved');
		expect(contained.reportedErrors).toEqual(['TypeError: createRadialGradient is not a function']);
	});

	test('freezes the gate when nothing contains it', async () => {
		const { uncontained } = await mountShellInBrowserEnvironment();
		expect(uncontained.gateText).not.toBe('signed-in-check-resolved');
	});
});

describe('the root layout', () => {
	const layout = readFileSync(new URL('../../../src/routes/+layout.svelte', import.meta.url), 'utf8');

	test('contains the app rail in an effect error boundary', () => {
		expect(layout).toMatch(/<EffectErrorBoundary region="app rail">\s*<AppRail[^>]*\/>\s*<\/EffectErrorBoundary>/);
	});

	test('renders the page through a boundary everywhere it renders children', () => {
		expect(layout.match(/\{@render children\(\)\}/g)?.length).toBe(1);
		expect(layout).toMatch(/\{#snippet contained\(\)\}\s*\{#key workspaceScope\}\s*<EffectErrorBoundary region="page">\s*\{@render children\(\)\}\s*<\/EffectErrorBoundary>\s*\{\/key\}/);
	});
});
