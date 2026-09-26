import { describe, expect, test } from 'bun:test';
import { hostAddressOf } from '../../src/lib/server/control-plane';
import { hostAddressOf as edgeHostAddressOf } from '../../../supabase/functions/_shared/host-address.ts';

describe('the company computer account', () => {
	test('is the same address to the app and to the edge functions', () => {
		const companyID = '00000000-0000-4000-8000-000000000001';
		expect(edgeHostAddressOf(companyID)).toBe(hostAddressOf(companyID));
	});
});
