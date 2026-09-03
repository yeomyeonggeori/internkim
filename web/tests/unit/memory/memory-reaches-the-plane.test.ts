import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import {
	episodeDeleteRequest,
	pinnedDeleteRequest,
	pinnedUpdateRequest
} from '../../../src/routes/memory/memory-graph-api';
import type { MemoryChange } from '../../../src/routes/memory/memory-change';
import {
	memoryScheduleListBody,
	scheduleCancelRequest,
	scheduleDeleteRequest,
	scheduleUpdateRequest
} from '../../../src/routes/memory/memory-schedule-api';

function relayCapabilityPaths(): Record<string, string> {
	const relay = readFileSync('../host/relay/forward.ts', 'utf8');
	const paths: Record<string, string> = {};
	for (const [, capability, path] of relay.matchAll(/'(person\.[a-z_.]+)': '([^']+)'/g)) {
		paths[capability] = path;
	}
	return paths;
}

const everyMemoryChange: MemoryChange[] = [
	scheduleCancelRequest('schedule-1'),
	scheduleDeleteRequest('schedule-2'),
	scheduleUpdateRequest('schedule-3', { name: '주간 보고', kind: 'cron', cronExpression: '0 9 * * 1' }),
	episodeDeleteRequest('episode-1', ['namespace-1', 'namespace-2']),
	pinnedUpdateRequest('금요일마다 회고'),
	pinnedDeleteRequest()
];

describe('what the memory screen asks the plane to change', () => {
	test('names a capability and the workspace path behind it, and carries only what was given', () => {
		expect(everyMemoryChange).toEqual([
			{
				capability: 'person.memory.schedule_cancel',
				path: '/memory/api/schedules/cancel',
				body: { taskScheduleID: 'schedule-1' }
			},
			{
				capability: 'person.memory.schedule_delete',
				path: '/memory/api/schedules/delete',
				body: { taskScheduleID: 'schedule-2' }
			},
			{
				capability: 'person.memory.schedule_update',
				path: '/memory/api/schedules/update',
				body: { taskScheduleID: 'schedule-3', name: '주간 보고', kind: 'cron', cronExpression: '0 9 * * 1' }
			},
			{
				capability: 'person.memory.episode_delete',
				path: '/memory/api/episodes/delete',
				body: { episodeID: 'episode-1', namespaceIDs: ['namespace-1', 'namespace-2'] }
			},
			{
				capability: 'person.memory.pinned_update',
				path: '/memory/api/pinned/update',
				body: { content: '금요일마다 회고' }
			},
			{ capability: 'person.memory.pinned_delete', path: '/memory/api/pinned/delete', body: {} }
		]);
	});

	test('is a capability the relay actually carries, at the same path', () => {
		const carried = relayCapabilityPaths();

		for (const change of everyMemoryChange) {
			expect(`${change.capability} ${carried[change.capability]}`).toBe(`${change.capability} ${change.path}`);
		}
	});
});

describe('what the memory screen asks the plane to read', () => {
	test('asks for nothing it was not given, so the workspace applies its own defaults', () => {
		expect(memoryScheduleListBody({})).toEqual({});
		expect(memoryScheduleListBody({ page: 0, pageSize: -1 })).toEqual({});
		expect(memoryScheduleListBody({ includeExpired: false })).toEqual({ includeExpired: false });
	});

	test('carries the page it was given, floored to whole rows', () => {
		expect(memoryScheduleListBody({ page: 2.7, pageSize: 25, includeExpired: true })).toEqual({
			page: 2,
			pageSize: 25,
			includeExpired: true
		});
	});

	test('reads the list and the graph through capabilities the relay carries', () => {
		const carried = relayCapabilityPaths();

		expect(carried['person.memory.schedules']).toBe('/memory/api/schedules');
		expect(carried['person.memory.graph']).toBe('/memory/api/graph');
	});
});
