import { describe, expect, test } from 'bun:test';
import { createTaskReadSession, readCoversWrite, sameTaskReadContext } from '../../../src/routes/task/task-read-session';

describe('task request ownership', () => {
	test('full history stays user-triggered through same-account route revalidation and week navigation', () => {
		const session = createTaskReadSession();
		session.select('company:member:member', '26W40');
		expect(session.needsFullHistory()).toBe(false);
		session.requireFullHistory();
		session.invalidate();
		session.select('company:member:member', '26W40');
		expect(session.needsFullHistory()).toBe(true);
		session.select('company:member:member', '26W41');
		expect(session.needsFullHistory()).toBe(true);
	});
	test('changing company, viewer or permission scope resets full demand', () => {
		for (const nextScope of ['other:member:member', 'company:other:member', 'company:member:admin']) {
			const session = createTaskReadSession();
			session.select('company:member:member', '26W40');
			session.requireFullHistory();
			const reading = session.start(1);
			session.select(nextScope, '26W40');
			expect(session.needsFullHistory()).toBe(false);
			expect(session.isCurrent(reading, 1)).toBe(false);
		}
	});
	test('a late response cannot install after a write, newer read, week change or disposal', () => {
		const session = createTaskReadSession();
		session.select('scope', '26W40');
		const beforeWrite = session.start(1);
		expect(session.isCurrent(beforeWrite, 1)).toBe(true);
		expect(session.isCurrent(beforeWrite, 2)).toBe(false);
		const beforeRefresh = session.start(2);
		expect(session.isCurrent(beforeWrite, 2)).toBe(false);
		session.select('scope', '26W41');
		expect(session.isCurrent(beforeRefresh, 2)).toBe(false);
		const beforeDisposal = session.start(2);
		session.invalidate();
		expect(session.isCurrent(beforeDisposal, 2)).toBe(false);
	});
	test('an early save must not wait for the pre-save directory before starting its reload', () => {
		const originalDirectory = { scope: 'scope', week: '26W40', generation: 1 };
		expect(sameTaskReadContext(originalDirectory, { ...originalDirectory })).toBe(true);
		expect(sameTaskReadContext(originalDirectory, { ...originalDirectory, generation: 2 })).toBe(false);
		expect(sameTaskReadContext(originalDirectory, { ...originalDirectory, week: '26W41' })).toBe(false);
		expect(sameTaskReadContext(originalDirectory, { ...originalDirectory, scope: 'other' })).toBe(false);
	});
	test('a current denial can finish its loading indicator after invalidating its own generation', () => {
		const session = createTaskReadSession();
		session.select('scope', '26W40');
		const reading = session.start(1);
		expect(session.isCurrent(reading, 1)).toBe(true);
		expect(session.isCurrent(reading, 2)).toBe(false);
		expect(session.owns(reading)).toBe(true);
	});
});

describe('a heard task write', () => {
	test('needs no second refresh when a read started after it and nothing invalidated that read', () => {
		expect(readCoversWrite(3, 2, 3)).toBe(true);
		expect(readCoversWrite(2, 2, 2)).toBe(true);
	});
	test('still refreshes when the only read predates it or was invalidated since', () => {
		expect(readCoversWrite(1, 2, 2)).toBe(false);
		expect(readCoversWrite(3, 2, 4)).toBe(false);
		expect(readCoversWrite(-1, 2, 2)).toBe(false);
	});
});
