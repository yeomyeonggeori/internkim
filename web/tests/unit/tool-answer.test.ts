import { describe, expect, test } from 'bun:test';
import { refusedToolOutcomes, resultOrRefusal, ToolRefused } from '../../src/lib/tool-answer';
import { ToolOutcome } from '../../src/lib/server/public-api/catalog/protocol';

function refusalOf(answer: () => unknown): ToolRefused {
	try {
		answer();
	} catch (refusal) {
		if (refusal instanceof ToolRefused) return refusal;
		throw refusal;
	}
	throw new Error('the answer was not refused');
}

describe('a tool answer', () => {
	test('hands back its result when the tool succeeded', () => {
		expect(resultOrRefusal('host_version_get', 200, { outcome: 'succeeded', result: { installedVersion: 'v1' } })).toEqual({
			installedVersion: 'v1'
		});
	});

	test('is a refusal when the company machine answered that the tool failed, though the call itself was carried', () => {
		const refusal = refusalOf(() =>
			resultOrRefusal('host_update', 200, {
				outcome: 'failed',
				errorCode: 'update_method_unsupported',
				message: 'the agent does not update a Mac host',
				result: {}
			})
		);
		expect([refusal.errorCode, refusal.message, refusal.status]).toEqual([
			'update_method_unsupported',
			'the agent does not update a Mac host',
			200
		]);
	});

	test('is a refusal when the call was not carried, in the words it was refused with', () => {
		expect(refusalOf(() => resultOrRefusal('host_version_get', 503, { error: 'server_offline' })).message).toBe('server_offline');
		expect(refusalOf(() => resultOrRefusal('host_version_get', 403, { message: 'this caller may not delete' })).message).toBe(
			'this caller may not delete'
		);
		expect(refusalOf(() => resultOrRefusal('host_version_get', 502, null)).message).toBe('host_version_get answered 502');
	});

	test('counts as refused every outcome the protocol has besides success', () => {
		expect([...refusedToolOutcomes].sort()).toEqual(
			Object.values(ToolOutcome)
				.filter((outcome) => outcome !== ToolOutcome.Succeeded)
				.sort()
		);
	});
});
