import { describe, expect, test } from 'bun:test';
import { dataRoomTemplate } from '../../../src/lib/data-room/model';
import { circleLabel, readableCategoryNames } from '../../../src/routes/data-room/circle-items';

const investor = {
	id: 'investor',
	name: 'Investor',
	isDefault: true,
	readableCategories: ['CR', 'CO'],
	memberIDs: [],
	channelName: null,
	channelID: null
};

describe('share link circle labels', () => {
	test('a link row names its circle in the reader language', () => {
		expect(circleLabel([investor], 'investor', 'ko')).toBe('투자자');
		expect(circleLabel([investor], 'investor', 'en')).toBe('Investor');
	});

	test('a circle the company made is named by its own name', () => {
		expect(circleLabel([{ ...investor, id: 'advisors', name: '자문단', isDefault: false }], 'advisors', 'ko')).toBe('자문단');
	});

	test('a circle no longer listed is still named by the template', () => {
		expect(circleLabel([], 'investor', 'ko')).toBe('투자자');
	});

	test('the circle scope names each category it reads', () => {
		expect(readableCategoryNames(investor, dataRoomTemplate.categories, 'ko')).toBe('설립·등록·정관, 주주·지분');
		expect(readableCategoryNames(investor, dataRoomTemplate.categories, 'en')).toBe('Registration, Ownership');
	});

	test('a code the company has no category for is shown as the code', () => {
		expect(readableCategoryNames({ ...investor, readableCategories: ['ZZ'] }, dataRoomTemplate.categories, 'ko')).toBe('ZZ');
	});
});
