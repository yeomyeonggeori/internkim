import { describe, expect, test } from 'bun:test';
import { sizeOfHours, sizeOfWholeDays, taskSizes } from '../../../src/lib/task/task-sizes';
import { WorkspaceTaskSize } from '../../../src/lib/server/public-api/catalog/workspace-task';
import taskSizeDefinitionsDocument from '../../../../internal/tasksize/definitions.json';

describe('sizeOfHours', () => {
	test('takes the smallest size the hours still fit in', () => {
		expect(sizeOfHours(0.5)).toBe('XS');
		expect(sizeOfHours(1)).toBe('XS');
		expect(sizeOfHours(1.5)).toBe('S');
		expect(sizeOfHours(2)).toBe('S');
		expect(sizeOfHours(3)).toBe('M');
		expect(sizeOfHours(8)).toBe('M');
		expect(sizeOfHours(9)).toBe('L');
		expect(sizeOfHours(16)).toBe('L');
		expect(sizeOfHours(17)).toBe('XL');
		expect(sizeOfHours(32)).toBe('XL');
	});

	test('anything longer than the largest size is still the largest', () => {
		expect(sizeOfHours(129)).toBe('XXL');
		expect(sizeOfHours(10000)).toBe('XXL');
	});
});

describe('sizeOfWholeDays', () => {
	test('a whole-day event is sized by days', () => {
		expect(sizeOfWholeDays(1)).toBe('M');
		expect(sizeOfWholeDays(2)).toBe('L');
		expect(sizeOfWholeDays(3)).toBe('XL');
		expect(sizeOfWholeDays(4)).toBe('XXL');
		expect(sizeOfWholeDays(30)).toBe('XXL');
	});

	test('less than a day is still a day', () => {
		expect(sizeOfWholeDays(0)).toBe('M');
	});
});

describe('taskSizes', () => {
	test('matches the canonical task-size definitions and public enum', () => {
		expect(taskSizes('en')).toEqual(taskSizeDefinitionsDocument.sizes.map((size) => ({
			name: size.name,
			distanceKm: size.distanceKm,
			maxHours: size.maxHours,
			score: size.score,
			label: `${size.distanceKm}km · up to ${size.maxHours}h`,
			...size.en
		})));
		expect(Object.values(WorkspaceTaskSize)).toEqual(taskSizeDefinitionsDocument.sizes.map((size) => size.name));
	});

	test('every size it can hand out is a size it defines', () => {
		const names = new Set(taskSizes().map((size) => size.name));
		for (const hours of [0.5, 2, 8, 16, 32, 500]) expect(names.has(sizeOfHours(hours))).toBe(true);
		for (const days of [1, 2, 3, 9]) expect(names.has(sizeOfWholeDays(days))).toBe(true);
	});
});
