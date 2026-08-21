import { describe, expect, test } from 'bun:test';
import { crmDefinitionLabel, crmLabel } from '../../../src/routes/crm/crm-labels';

describe('crmDefinitionLabel', () => {
	const definitions = [
		{ id: 'def-8f2k1x', name: '핵심 파트너' },
		{ id: 'partner', name: 'partner' }
	];

	test('shows the vocabulary name for a generated definition id', () => {
		expect(crmDefinitionLabel(definitions, { partner: '파트너' }, 'def-8f2k1x')).toBe('핵심 파트너');
	});

	test('falls back to the static table when the name is just the id', () => {
		expect(crmDefinitionLabel(definitions, { partner: '파트너' }, 'partner')).toBe('파트너');
	});

	test('shows the raw value when nothing knows it', () => {
		expect(crmDefinitionLabel(definitions, {}, 'unknown')).toBe('unknown');
	});
});

describe('crmLabel', () => {
	test('keeps the static lookup behaviour', () => {
		expect(crmLabel({ meeting: '회의' }, 'meeting')).toBe('회의');
		expect(crmLabel({}, 'meeting')).toBe('meeting');
	});

	test('shows a value the dictionary has no wording for', () => {
		const translations = new Proxy({}, { get: (_target, key) => (key === 'meeting' ? '회의' : '') });

		expect(crmLabel(translations, 'meeting')).toBe('회의');
		expect(crmLabel(translations, '통화')).toBe('통화');
	});
});
