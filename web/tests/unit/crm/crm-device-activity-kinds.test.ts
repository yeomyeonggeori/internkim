import { describe, expect, test } from 'bun:test';
import { deviceCRMActivityKinds } from '../../../src/routes/crm/crm-types';

const validationSource = '../../../../internal/admind/crm_http_validation.go';

async function kindsAllowedByTheDevice(): Promise<string[]> {
	const source = await Bun.file(new URL(validationSource, import.meta.url)).text();
	const allowed = source.match(/crmValueAllowed\(kind,\s*([^)]*)\)/);
	if (!allowed) throw new Error('the device no longer states its activity kinds through crmValueAllowed');
	return [...allowed[1]!.matchAll(/"([^"]+)"/g)].map((match) => match[1]!);
}

describe('device CRM activity kinds', () => {
	test('match the kinds the device accepts', async () => {
		expect(deviceCRMActivityKinds).toEqual(await kindsAllowedByTheDevice());
	});

	test('leave the automatic stage change out of what a person may choose', () => {
		expect(deviceCRMActivityKinds).not.toContain('stage_change');
	});
});
