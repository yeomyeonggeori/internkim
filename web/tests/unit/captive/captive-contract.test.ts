import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { joinForm, joinOutcomeFor, parseNetworkListing, type JoinOutcome, type NetworkListing } from '../../../captive/captive-api';

type AnsweredOutcome = Exclude<JoinOutcome, 'unreachable'>;

type CaptiveContract = {
	networksResponse: NetworkListing;
	joinFields: string[];
	joinStatuses: Record<AnsweredOutcome, number>;
};

const contract: CaptiveContract = JSON.parse(
	readFileSync(new URL('../../../../internal/boxwifi/testdata/captive-contract.json', import.meta.url), 'utf8')
);

describe('the setup page reads what the box answers', () => {
	test('parses the network listing the box sends', () => {
		expect(parseNetworkListing(contract.networksResponse)).toEqual(contract.networksResponse);
	});

	test('posts the join form under the field names the box reads', () => {
		expect([...joinForm('Office', '', 'secret').keys()]).toEqual(contract.joinFields);
	});

	test('maps each join status the box answers to its outcome', () => {
		const outcomes: AnsweredOutcome[] = ['accepted', 'networkRequired', 'alreadySubmitted'];
		for (const outcome of outcomes) {
			expect(joinOutcomeFor(contract.joinStatuses[outcome])).toBe(outcome);
		}
	});

	test('refuses a listing whose fields were renamed', () => {
		expect(() => parseNetworkListing({ networks: [{ name: 'Office' }], hasJoinFailed: false })).toThrow();
	});
});
