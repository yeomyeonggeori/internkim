import { describe, expect, test } from 'bun:test';
import { defaultCreditUsageLimit } from '../../src/lib/credit-policy';
import { defaultCreditUsageLimit as gatewayDefaultCreditUsageLimit } from '../../../supabase/functions/_shared/credit-policy.ts';

describe('the default credit usage limit is one number', () => {
	test('the settings screen shows the limit the gateway enforces', () => {
		expect(defaultCreditUsageLimit).toBe(gatewayDefaultCreditUsageLimit);
	});
});
