import { describe, expect, test } from 'bun:test';

import { companyConnectionKinds } from '../../../src/lib/company/connections';
import { messengerPlatformNames } from '../../../src/lib/server/public-api/catalog/protocol';
import { companySettingsText } from '../../../src/routes/settings/text';

describe('the connections a company keeps', () => {
	test('name only messengers the protocol declares', () => {
		const undeclared = companyConnectionKinds.filter(
			(kind) => !messengerPlatformNames.some((declared) => declared === kind)
		);

		expect(undeclared).toEqual([]);
	});

	test('include the messenger this product runs on', () => {
		expect(companyConnectionKinds).toContain('buzz');
	});

	test('are each named and described in every language', () => {
		for (const locale of Object.keys(companySettingsText)) {
			const text = companySettingsText[locale as keyof typeof companySettingsText];
			for (const kind of companyConnectionKinds) {
				expect(text[kind]).toBeTruthy();
				expect(text[`${kind}Description`]).toBeTruthy();
			}
		}
	});
});
