import { describe, expect, test } from 'bun:test';
import { apiReferenceHomeFor } from '../../../src/lib/server/api-reference-redirect';

describe('the API reference has one home', () => {
	test('the reference page and its languages go to the docs site', () => {
		expect(apiReferenceHomeFor('/api-docs')).toBe('https://docs.intern.kim/api');
		expect(apiReferenceHomeFor('/api-docs/ko')).toBe('https://docs.intern.kim/api');
		expect(apiReferenceHomeFor('/api-docs/en')).toBe('https://docs.intern.kim/api');
	});

	test('the document keeps its language where it lands', () => {
		expect(apiReferenceHomeFor('/openapi/en.json')).toBe('https://docs.intern.kim/openapi/en.json');
		expect(apiReferenceHomeFor('/openapi/ko.json')).toBe('https://docs.intern.kim/openapi/ko.json');
	});

	test('a language nobody publishes is not sent anywhere', () => {
		expect(apiReferenceHomeFor('/openapi/fr.json')).toBeNull();
		expect(apiReferenceHomeFor('/openapi')).toBeNull();
	});

	test('the app itself is left alone', () => {
		expect(apiReferenceHomeFor('/settings')).toBeNull();
		expect(apiReferenceHomeFor('/api/member/tokens')).toBeNull();
		expect(apiReferenceHomeFor('/api-docsomething')).toBeNull();
	});

	test('a self-hosted zone replaces the default', () => {
		expect(apiReferenceHomeFor('/api-docs', 'example.test')).toBe('https://docs.example.test/api');
		expect(apiReferenceHomeFor('/openapi/en.json', 'example.test')).toBe(
			'https://docs.example.test/openapi/en.json'
		);
	});
});
