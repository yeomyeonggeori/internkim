import { describe, expect, test } from 'bun:test';
import { endpointStatusLabel, statusBadgeVariant } from '../../../src/routes/ops/ops-view';

describe('ops view helpers', () => {
	test('maps healthy states to non-destructive badges', () => {
		expect(statusBadgeVariant('ok')).toBe('secondary');
		expect(statusBadgeVariant('running')).toBe('outline');
		expect(statusBadgeVariant('auth_required')).toBe('outline');
	});

	test('maps failed states to destructive badges', () => {
		expect(statusBadgeVariant('failed')).toBe('destructive');
	});

	test('formats endpoint status with HTTP code', () => {
		expect(endpointStatusLabel({ state: 'ok', code: 200 })).toBe('ok 200');
		expect(endpointStatusLabel(undefined)).toBe('unknown');
	});
});
