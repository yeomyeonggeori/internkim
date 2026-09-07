import { describe, expect, test } from 'bun:test';
import { centralTaskStatusOptions } from '../../src/lib/task/central-task';
import { taskStatus, taskStatusLabelFrom } from '../../src/routes/task/task-status';
import { taskStatusIcon, taskStatusRank, taskStatusVariant } from '../../src/routes/task/task-status-style';
import { statusSelectOptions } from '../../src/routes/task/task-options';
import { taskText } from '../../src/routes/task/text';
import { taskStatusLabel as taskRunStatusLabel } from '../../src/routes/runs/runs-view';
import { tasksText } from '../../src/routes/runs/text';

const vocabularies = { ko: taskText.ko.status, en: taskText.en.status } as Record<string, Record<string, string>>;

describe('the word every status shows a reader', () => {
	for (const [locale, labels] of Object.entries(vocabularies)) {
		test(`${locale} names every status the runtime can produce`, () => {
			for (const status of Object.values(taskStatus)) {
				expect(taskStatusLabelFrom(labels, status)).not.toBe(status);
			}
		});

		test(`${locale} names every status the central plane offers`, () => {
			for (const status of centralTaskStatusOptions) {
				expect(taskStatusLabelFrom(labels, status)).not.toBe(status);
			}
		});
	}

	for (const [locale, labels] of Object.entries(vocabularies)) {
		test(`${locale} names every status a list row can offer`, () => {
			const options = statusSelectOptions(centralTaskStatusOptions, (status) => taskStatusLabelFrom(labels, status));
			for (const option of options) {
				expect(option.label).not.toBe(option.value);
			}
		});
	}

	test('reads a status whatever case it arrives in', () => {
		expect(taskStatusLabelFrom(taskText.ko.status, 'Completed')).toBe('완료');
		expect(taskStatusLabelFrom(taskText.ko.status, 'COMPLETED')).toBe('완료');
		expect(taskStatusLabelFrom(taskText.en.status, 'In_Progress')).toBe('In progress');
	});

	test('classifies a status whatever case it arrives in', () => {
		expect(taskStatusIcon('Completed')).toBe(taskStatusIcon('completed'));
		expect(taskStatusVariant('IN_PROGRESS')).toBe(taskStatusVariant('in_progress'));
		expect(taskStatusRank(' Stopped ')).toBe(taskStatusRank('stopped'));
	});

	test('reads a status that carries stray whitespace', () => {
		expect(taskStatusLabelFrom(taskText.ko.status, ' completed ')).toBe('완료');
	});

	test('shows the word itself when nothing names it', () => {
		expect(taskStatusLabelFrom(taskText.ko.status, 'invented')).toBe('invented');
	});
});

const taskRunStatuses = [
	'completed',
	'failed',
	'running',
	'planned',
	'waiting_user_input',
	'waiting_approval',
	'blocked',
	'interrupted',
	'cancelled'
];

describe('the word every task run status shows a reader', () => {
	for (const [locale, text] of Object.entries({ ko: tasksText.ko, en: tasksText.en })) {
		test(`${locale} names every status the run list can show`, () => {
			for (const status of taskRunStatuses) {
				expect(taskRunStatusLabel(status, text)).not.toBe(status);
			}
		});

		test(`${locale} names every status the run list offers as a tab`, () => {
			for (const label of [text.statusAll, text.statusFailed, text.statusRunning, text.statusCompleted]) {
				expect(taskRunStatuses).not.toContain(label);
			}
		});
	}
});
