import { afterEach, expect, mock, spyOn, test } from 'bun:test';
import { changedTaskFields, saveTask } from '../../../src/lib/task/task-state';
import * as record from '../../../src/lib/task/task-record';
import * as persistence from '../../../src/routes/task/task-persistence';
import { TaskEditorController } from '../../../src/routes/task/task-editor-controller.svelte';
import type { Task } from '../../../src/routes/task/task-types';

function task(): Task {
	return {
		id: 'task-one', ownerID: 'member-one', ownerName: 'Sample',
		participantIDs: ['member-one', 'member-two'], participantNames: ['Sample', 'Example'],
		business: 'Business', type: 'Review', content: 'Original title', size: 'M',
		status: 'planned', startDate: '2026-10-01', endDate: '2026-10-02', weekCode: '26W40'
	};
}

afterEach(() => mock.restore());

test('title edits leave unrelated dates, status, labels, and participants untouched', () => {
	const original = task();
	expect(changedTaskFields({ ...original, content: 'Edited title' }, original)).toEqual({ title: 'Edited title' });
});

test('list status edits contain only the changed status', () => {
	const original = task();
	expect(changedTaskFields({ ...original, status: 'completed' }, original)).toEqual({ status: 'completed' });
});

test('copied participant arrays do not become replacement writes', () => {
	const original = task();
	expect(changedTaskFields({ ...original, participantIDs: [...original.participantIDs] }, original)).toEqual({});
});

test('explicit participant edits preserve every selected identifier', () => {
	const original = task();
	expect(changedTaskFields({ ...original, participantIDs: ['member-one', 'member-three'] }, original)).toEqual({
		participantPersonHints: ['member-one', 'member-three']
	});
});

test('clearing labels and dates sends explicit empty fields', () => {
	const original = task();
	expect(changedTaskFields({ ...original, business: null, type: null, startDate: undefined, endDate: undefined }, original)).toEqual({
		business: '', type: '', startsAt: '', endsAt: ''
	});
});

test('an original snapshot cannot be used for another task', () => {
	expect(() => changedTaskFields({ ...task(), id: 'another-task' }, task())).toThrow('original task');
});

test('saving an existing edit transmits only changed fields', async () => {
	const original = task();
	const update = spyOn(record, 'updateTask').mockResolvedValue({ taskID: original.id });
	await saveTask({ ...original, content: 'Edited title' }, original.status, original);
	expect(update).toHaveBeenCalledWith(original.id, { title: 'Edited title' });
});

test('saving an unchanged draft performs no update', async () => {
	const original = task();
	const update = spyOn(record, 'updateTask').mockResolvedValue({ taskID: original.id });
	await saveTask({ ...original, participantIDs: [...original.participantIDs] }, original.status, original);
	expect(update).not.toHaveBeenCalled();
});

test('the editor keeps a separate original snapshot while a draft changes', async () => {
	const state = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	try {
		const original = task();
		const editor = new TaskEditorController();
		const save = spyOn(persistence, 'saveTaskDraft').mockResolvedValue({ status: 'saved' });
		editor.openTask(original, () => false);
		if (!editor.taskDraft) throw new Error('the editor did not open');
		editor.taskDraft.content = 'Edited title';
		original.participantIDs.push('member-three');
		await editor.saveTask();
		expect(save.mock.calls[0]?.[0].originalTask).toEqual(task());
		expect(save.mock.calls[0]?.[0].task?.content).toBe('Edited title');
	} finally {
		if (state === undefined) Reflect.deleteProperty(globalThis, '$state');
		else Reflect.set(globalThis, '$state', state);
	}
});
