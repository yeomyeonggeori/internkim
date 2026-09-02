import { describe, expect, test } from 'bun:test';
import webPackage from '../../../package.json';
import protocolPackage from '../../../../.dependency/blueclaw/protocol/package.json';

describe('the two zod pins', () => {
	test('are the same exact version', () => {
		const webPin = webPackage.devDependencies.zod;
		const protocolPin = protocolPackage.dependencies.zod;

		expect(webPin).toBe(protocolPin);
		expect(webPin).toMatch(/^\d+\.\d+\.\d+$/);
	});
});
