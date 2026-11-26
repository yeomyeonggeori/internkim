import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

interface ShellOutcome {
	failureText: string | null;
	pageText: string | null;
	gateText: string | null;
	reportedErrors: string[];
	thrown: string | null;
}

async function mountShellInBrowserEnvironment(): Promise<{ contained: ShellOutcome; uncontained: ShellOutcome; page: ShellOutcome; left: ShellOutcome }> {
	return JSON.parse(await runInItsOwnProcess(new URL('./effect-error-boundary.fixture.ts', import.meta.url), 'browser'));
}

describe('an effect that throws inside the app rail', () => {
	test('stays inside its boundary, is reported once, and lets the sign-in gate resolve', async () => {
		const { contained } = await mountShellInBrowserEnvironment();
		expect(contained.gateText).toBe('signed-in-check-resolved');
		expect(contained.reportedErrors).toEqual(['TypeError: createRadialGradient is not a function']);
	}, ownProcessTimeoutMilliseconds);

	test('freezes the gate when nothing contains it', async () => {
		const { uncontained } = await mountShellInBrowserEnvironment();
		expect(uncontained.gateText).not.toBe('signed-in-check-resolved');
	}, ownProcessTimeoutMilliseconds);
});

describe('an effect that throws inside the page', () => {
	test('is replaced by the notice the page was given, and is still reported', async () => {
		const { page } = await mountShellInBrowserEnvironment();
		expect(page.failureText).toBe('page-failed');
		expect(page.reportedErrors).toEqual(['TypeError: createRadialGradient is not a function']);
	}, ownProcessTimeoutMilliseconds);

	test('gives way to the next page once the person leaves the failed one', async () => {
		const { left } = await mountShellInBrowserEnvironment();
		expect(left.failureText).toBeNull();
		expect(left.pageText).toBe('next-page');
	}, ownProcessTimeoutMilliseconds);

	test('leaves a region that was given no notice empty', async () => {
		const { contained } = await mountShellInBrowserEnvironment();
		expect(contained.failureText).toBeNull();
	}, ownProcessTimeoutMilliseconds);
});
