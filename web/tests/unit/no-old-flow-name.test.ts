import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// The board was `flow`, renamed to `task` in d9ea9e618. What is left answering
// to the old name is either the bookmarked `/flow` redirect (web/src/routes/flow)
// or a persisted contract this issue does not touch: admind's SQLite tables,
// Mattermost post props, the relay's queue kinds, and capabilityd's
// `flow.task.*` tool aliases (internal/capabilities/legacy.go) all live outside
// web, tools and docs, so a scan here never sees them. Everything this file
// allows past the scan is one of those, named at the line it excuses.

const scanRoots = ['src', 'scripts', '../tools', '../docs'];
const skipDirectories = new Set(['node_modules', '.svelte-kit', 'build', 'dist', '.wrangler', '.turbo', '.git', 'private']);
const skipPathSegments = ['src' + sep + 'routes' + sep + 'flow'];
const binaryExtensions = new Set(['.png', '.jpg', '.jpeg', '.gif', '.ico', '.woff', '.woff2', '.ttf', '.pdf', '.svg', '.webp']);

const wholeFileAllowed = new Set([
	// The multi-turn ask.input driver for a company-table conversation; "flow"
	// here is the conversation's flow, unrelated to the renamed task board.
	'../tools/e2e-ask-flow',
	'../tools/e2e-ask-flow-remote.sh'
]);

const allowedLineSnippets: Record<string, string[]> = {
	'../tools/e2e-crud': ['start/approve/finish flow'],
	'../tools/e2e-crud-remote.sh': ['state/flow.sqlite', '/flow/|:8065', 'approval flow'],
	'../docs/internal/architecture.md': ['Flow, 일정'],
	'../docs/internal/blueclaw-oss-boundary.md': ['`flow.*`'],
	'../docs/internal/calendar-retirement-design.md': ['`flow` tasks'],
	'../docs/internal/nothing-reaches-in.md': ['`/admin`, `/flow`', 'intern.kim/flow/'],
	'../docs/internal/plans/2026-08-12-task-parent-relationship.md': ['no Flow types'],
	'../docs/internal/public-api-on-the-plane.md': ["capabilityd's flow tool"],
	'../docs/internal/saas-design.md': [
		'company/flow/site tools',
		'flow/tasks, memory',
		'Onboarding flow',
		'headless flow',
		'the approval flow (`user.confirm`'
	],
	'../docs/internal/skill-orchestration-design.md': ['flow.task.add'],
	'../docs/internal/task-sync-direction.md': [
		'its flow tables',
		'flow task mapping',
		"admind's flow HTTP API",
		"the agent's flow tools in capabilityd",
		"admind's flow read path",
		'flow-date-repair',
		'flow-compare-central'
	],
	'scripts/import-task-state.ts': ['host/relay/flow-task-as-task'],
	'src/lib/app-shell.ts': ["'/flow/'"],
	'src/lib/company-path.ts': ["'flow',"],
	'src/routes/company/company-page-text.ts': ['Recent work flow'],
	'src/routes/company/team-activity-section.svelte': ['grid-auto-flow'],
	'src/routes/crm/text.ts': ['flow of opportunities']
};

function filesUnder(directory: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(directory)) {
		if (skipDirectories.has(entry)) continue;
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) {
			found.push(...filesUnder(path));
			continue;
		}
		const extension = entry.includes('.') ? entry.slice(entry.lastIndexOf('.')).toLowerCase() : '';
		if (binaryExtensions.has(extension)) continue;
		found.push(path);
	}
	return found;
}

function isSkippedPath(path: string): boolean {
	return skipPathSegments.some((segment) => path.includes(segment));
}

describe('the old flow name', () => {
	test('appears only where the redirect or a persisted contract needs it', () => {
		const oldNameWord = /\bflow\b/i;
		const violations: string[] = [];

		for (const root of scanRoots) {
			for (const path of filesUnder(root)) {
				if (isSkippedPath(path)) continue;
				if (wholeFileAllowed.has(path)) continue;
				const allowed = allowedLineSnippets[path] ?? [];
				const lines = readFileSync(path, 'utf8').split('\n');
				lines.forEach((line, index) => {
					if (!oldNameWord.test(line)) return;
					if (allowed.some((snippet) => line.includes(snippet))) return;
					violations.push(`${relative('.', path)}:${index + 1}: ${line.trim()}`);
				});
			}
		}

		expect(violations).toEqual([]);
	});
});
