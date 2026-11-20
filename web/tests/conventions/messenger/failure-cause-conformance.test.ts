import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { failureCauses } from '../../../src/lib/messenger/failure-cause';

const failureCauseSource = readFileSync(
	new URL('../../../../.dependency/blueclaw/.dependency/blueprotocol/agentcontract/failure_cause.go', import.meta.url),
	'utf8'
);

function runtimeFailureCauses(): string[] {
	return [...failureCauseSource.matchAll(/FailureCause\w+\s+FailureCause\s*=\s*"([^"]+)"/g)].map((match) => match[1]);
}

describe('the failure causes are one list', () => {
	test('the messenger frames every cause the runtime can name', () => {
		expect(runtimeFailureCauses().length).toBeGreaterThan(0);
		const screenFailureCauses: string[] = [...failureCauses];
		expect(screenFailureCauses.sort()).toEqual(runtimeFailureCauses().sort());
	});
});
