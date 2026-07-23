import { describe, expect, test } from 'bun:test';
import { createDevPersonProfileImageMockResponse } from '../../dev-person-profile-image-mock-plugin';

describe('createDevPersonProfileImageMockResponse', () => {
	test('returns a missing image response for a participant profile image request', () => {
		expect(
			createDevPersonProfileImageMockResponse(
				'GET',
				'/calendar/api/participants/person%2Fkim/image'
			)
		).toEqual({ status: 404 });
	});

	test('ignores non-image calendar participant requests', () => {
		expect(
			createDevPersonProfileImageMockResponse('GET', '/calendar/api/participants')
		).toBe(undefined);
		expect(
			createDevPersonProfileImageMockResponse(
				'GET',
				'/calendar/api/participants/person-kim/image/metadata'
			)
		).toBe(undefined);
	});

	test('ignores profile image requests with unsupported methods', () => {
		expect(
			createDevPersonProfileImageMockResponse(
				'POST',
				'/calendar/api/participants/person-kim/image'
			)
		).toBe(undefined);
	});
});
