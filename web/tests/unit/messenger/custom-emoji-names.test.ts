import { describe, expect, test } from 'bun:test';
import { customEmojiNamesIn, shortcodeNamesIn } from '../../../src/lib/messenger/custom-emoji-names';

function postOf(body: string, ...reactions: string[]) {
	return { body, reactions: reactions.map((emoji) => ({ emoji })) };
}

describe('the names a page of messages actually references', () => {
	test('a shortcode in the body is one name', () => {
		expect(shortcodeNamesIn('good work :party_parrot: everyone')).toEqual(['party_parrot']);
	});

	test('a colon with a space in it is not a shortcode', () => {
		expect(shortcodeNamesIn('11:30 to 12:00, see :the plan: below')).toEqual([]);
	});

	test('reactions count as references, so a reaction still draws', () => {
		expect(customEmojiNamesIn([postOf('done', 'shipit')])).toEqual(['shipit']);
	});

	test('a page names only what it holds, however large the set behind it is', () => {
		const page = [postOf('ship :rocket:', 'tada'), postOf('nothing here')];

		expect(new Set(customEmojiNamesIn(page))).toEqual(new Set(['rocket', 'tada']));
	});

	test('an empty page names nothing to draw', () => {
		expect(customEmojiNamesIn([])).toEqual([]);
	});
});
