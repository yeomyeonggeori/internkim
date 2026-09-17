import { describe, expect, test } from 'bun:test';
import { hostnamesToAnswerFor, routeCoversHostname } from '../../../scripts/pages-hostnames';

describe('which Workers routes stand in front of a hostname', () => {
	test('a route names the hostname exactly, with or without a path', () => {
		expect(routeCoversHostname('api.intern.kim/v1/*', 'api.intern.kim')).toBe(true);
		expect(routeCoversHostname('api.intern.kim/v1', 'api.intern.kim')).toBe(true);
		expect(routeCoversHostname('updates.intern.kim/*', 'api.intern.kim')).toBe(false);
	});

	test('a wildcard host covers every name it matches and nothing else', () => {
		expect(routeCoversHostname('*.intern.kim/*', 'api.intern.kim')).toBe(true);
		expect(routeCoversHostname('*.intern.kim/*', 'intern.kim')).toBe(false);
		expect(routeCoversHostname('*intern.kim/*', 'notintern.kim')).toBe(true);
	});

	test('a scheme in the pattern does not change the host', () => {
		expect(routeCoversHostname('https://api.intern.kim/*', 'api.intern.kim')).toBe(true);
	});
});

describe('which hostnames a build must answer at', () => {
	test('the project address first, then every active custom domain', () => {
		expect(
			hostnamesToAnswerFor('internkim', [
				{ name: 'intern.kim', status: 'active' },
				{ name: 'staging.intern.kim', status: 'pending' }
			])
		).toEqual(['internkim.pages.dev', 'intern.kim']);
	});
});
