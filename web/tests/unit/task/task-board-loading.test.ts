import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

test('scoped task reads keep their API mocks isolated', async () => {
	const fixture = new URL('task-board-loading.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, 'test', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, `${output}\n${errors}`).toBe(0);
});

test('the task route requests only its selected board and lazily imports secondary panes', () => {
	const route = readFileSync(new URL('../../../src/routes/task/+page.ts', import.meta.url), 'utf8');
	const page = readFileSync(new URL('../../../src/routes/task/+page.svelte', import.meta.url), 'utf8');
	expect(route).toContain('fetchTaskState(taskScope, taskWeek.startISO)');
	expect(route).not.toContain('fetchTaskState(taskScope)');
	for (const pane of ['task-report-view', 'task-members-view', 'task-definitions-editor', 'task-report-sections-model']) {
		expect(page).toContain(`import('./${pane}`);
		expect(page).not.toMatch(new RegExp(`import [^;\\n]+ from ['"]\\./${pane}`));
	}
});

test('history demand waits for directory validation and stale definition writes stay guarded', () => {
	const page = readFileSync(new URL('../../../src/routes/task/+page.svelte', import.meta.url), 'utf8');
	const definitions = readFileSync(new URL('../../../src/routes/task/task-definitions-editor.svelte', import.meta.url), 'utf8');
	expect(page).toContain('if (boardRequest && sameTaskReadContext(boardRequest, context) && !taskState?.peopleReady)');
	expect(page).toContain('if (readSession.needsFullHistory())');
	expect(page).toContain('isFresh={isStateFresh && !isLoading}');
	expect(definitions).toContain('const canEditDefinitions = () => isFresh');
	expect(definitions).toContain('if (!summary?.isAdmin || !canEditDefinitions()) return');
});
