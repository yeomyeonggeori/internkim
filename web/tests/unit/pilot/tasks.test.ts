import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { isDeliveredFileAssertion, isRecordAssertion, loadTasks, type PilotTask } from '../../pilot/record';

const repositoryRoot = resolve(import.meta.dir, '../../../..');
const tasksDirectory = resolve(import.meta.dir, '../../pilot/tasks');
const seed = readFileSync(join(repositoryRoot, 'supabase/seed.dev.sql'), 'utf8');
const filenames = readdirSync(tasksDirectory).filter((name) => name.endsWith('.json')).sort();
const tasks = loadTasks(tasksDirectory);

function tablesInMigrations(): Set<string> {
	const directory = join(repositoryRoot, 'supabase/migrations');
	const tables = new Set<string>();
	for (const name of readdirSync(directory).sort()) {
		const source = readFileSync(join(directory, name), 'utf8');
		for (const found of source.matchAll(/create table public\.([a-z_]+)/g)) tables.add(found[1]);
		for (const found of source.matchAll(/alter table public\.([a-z_]+) rename to ([a-z_]+)/g)) {
			tables.delete(found[1]);
			tables.add(found[2]);
		}
		for (const found of source.matchAll(/drop table public\.([a-z_]+)/g)) tables.delete(found[1]);
	}
	return tables;
}

function identitiesIn(task: PilotTask): string[] {
	const identity = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/g;
	return [...JSON.stringify(task).matchAll(identity)].map((found) => found[0]);
}

function isLong(task: PilotTask): boolean {
	return task.assertions.some(isDeliveredFileAssertion);
}

describe('the thirty task program', () => {
	test('the directory holds thirty tasks numbered in order', () => {
		expect(tasks).toHaveLength(30);
		expect(filenames.map((name) => name.slice(0, 2))).toEqual(
			Array.from({ length: 30 }, (_, index) => String(index + 1).padStart(2, '0')),
		);
	});

	test('every task name is its own', () => {
		expect([...new Set(tasks.map((task) => task.name))]).toHaveLength(tasks.length);
	});

	test('every file is named after the task it holds', () => {
		const mismatched = filenames.filter((name, index) => name !== `${name.slice(0, 2)}-${tasks[index].name}.json`);
		expect(mismatched).toEqual([]);
	});

	test('every task carries an instruction, an assertion and a cleanup step', () => {
		const incomplete = tasks
			.filter((task) => task.instruction.trim() === '' || task.assertions.length === 0 || task.cleanup.length === 0)
			.map((task) => task.name);
		expect(incomplete).toEqual([]);
	});

	test('at least ten tasks are long enough to deliver a file', () => {
		expect(tasks.filter(isLong).length).toBeGreaterThanOrEqual(10);
	});

	test('every identity a task names exists in the seed', () => {
		const unknown = tasks.flatMap((task) => identitiesIn(task).filter((identity) => !seed.includes(identity)).map((identity) => `${task.name}: ${identity}`));
		expect(unknown).toEqual([]);
	});

	test('every requester a task names is a seeded member', () => {
		const unknown = tasks
			.filter((task) => task.requesterEmail !== undefined && !seed.includes(task.requesterEmail))
			.map((task) => `${task.name}: ${task.requesterEmail}`);
		expect(unknown).toEqual([]);
	});

	test('every record assertion reads a table the schema creates', () => {
		const tables = tablesInMigrations();
		const unknown = tasks.flatMap((task) =>
			task.assertions
				.filter(isRecordAssertion)
				.filter((assertion) => !tables.has(assertion.table))
				.map((assertion) => `${task.name}: ${assertion.table}`),
		);
		expect(unknown).toEqual([]);
	});

	test('every cleanup step writes a table the schema creates', () => {
		const tables = tablesInMigrations();
		const unknown = tasks.flatMap((task) =>
			task.cleanup
				.map((step) => ('delete' in step ? step.delete : step.update))
				.filter((table) => !tables.has(table))
				.map((table) => `${task.name}: ${table}`),
		);
		expect(unknown).toEqual([]);
	});
});
