import { describe, expect, test } from 'bun:test';

import { credentialKinds } from '../../../src/lib/server/public-api/catalog/credential';

describe('the kinds a credential is one of', () => {
	test('are each spelled once, so a person and their company never share a row', () => {
		expect(new Set(credentialKinds).size).toBe(credentialKinds.length);
	});
});
