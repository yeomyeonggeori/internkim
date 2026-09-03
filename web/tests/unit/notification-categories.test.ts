import { describe, expect, test } from 'bun:test';
import { notificationCategories } from '../../src/lib/notifications/categories';
import {
	choicesFromStored,
	storedFromChoices
} from '../../src/lib/server/public-api/record/notifications';

describe('what a member is told about', () => {
	test('a member who has chosen nothing hears everything except mail', () => {
		const chosen = choicesFromStored(null);
		expect(chosen.message).toBe(true);
		expect(chosen.task).toBe(true);
		expect(chosen.approval).toBe(true);
		expect(chosen.attendance).toBe(true);
		expect(chosen.leave).toBe(true);
		expect(chosen.calendar).toBe(true);
		expect(chosen.mail).toBe(false);
	});

	test('a stored choice wins over the default', () => {
		const chosen = choicesFromStored({ message: false, mail: true });
		expect(chosen.message).toBe(false);
		expect(chosen.mail).toBe(true);
		expect(chosen.task).toBe(true);
	});

	test('a value that is not a boolean is not a choice', () => {
		const chosen = choicesFromStored({ message: 'no', task: 1, approval: null });
		expect(chosen.message).toBe(true);
		expect(chosen.task).toBe(true);
		expect(chosen.approval).toBe(true);
	});

	test('what is written back is what reading it gives again', () => {
		const chosen = choicesFromStored({ message: false, mail: true });
		expect(choicesFromStored(storedFromChoices(chosen))).toEqual(chosen);
	});

	test('every category is written, so a new one does not read as a refusal', () => {
		const stored = storedFromChoices(choicesFromStored(null));
		for (const category of notificationCategories) {
			expect(typeof stored[category]).toBe('boolean');
		}
	});
});
