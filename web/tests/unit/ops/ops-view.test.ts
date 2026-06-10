import { describe, expect, test } from 'bun:test';
import { endpointStatusLabel, readableModelLabel, runtimeVersionDetail, shortRelease, shortVersion, statusBadgeVariant } from '../../../src/routes/ops/ops-view';

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
		expect(endpointStatusLabel({ state: 'ok', code: 200 })).toBe('OK 200');
		expect(endpointStatusLabel({ state: 'not_json', code: 200 })).toBe('Wrong route 200');
		expect(endpointStatusLabel(undefined)).toBe('Unknown');
	});

	test('shortens long component revisions', () => {
		expect(shortVersion('abcdefghijklmnopqrstuvwxyz')).toBe('abcdefghijkl');
		expect(shortVersion('')).toBe('Unchanged');
		expect(shortRelease('')).toBe('No release');
	});

	test('formats model labels', () => {
		expect(readableModelLabel({ state: 'ok', model: 'x-ai/grok-4.3' })).toBe('x-ai/grok-4.3');
		expect(readableModelLabel({ state: 'failed' })).toBe('Unreadable');
	});

	test('formats runtime component detail', () => {
		const detail = runtimeVersionDetail({
			capabilityd: 'capabilityd-revision',
			blueclawPayload: 'payload-revision',
			skills: 'skills-revision'
		});
		expect(detail.includes('payload: payload-revision')).toBe(true);
	});
});
