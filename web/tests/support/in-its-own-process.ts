import { expect } from 'bun:test';

export const ownProcessTimeoutMilliseconds = 60_000;

type ProcessKind = 'script' | 'browser' | 'tests';

const leadingArguments: Record<ProcessKind, string[]> = {
	script: [],
	browser: ['--conditions', 'browser'],
	tests: ['test']
};

export async function runInItsOwnProcess(fixture: URL, kind: ProcessKind, ...fixtureArguments: string[]): Promise<string> {
	const run = Bun.spawn([process.execPath, ...leadingArguments[kind], fixture.pathname, ...fixtureArguments], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited]);
	expect(exitCode, `${output}\n${errors}`).toBe(0);
	return output;
}
