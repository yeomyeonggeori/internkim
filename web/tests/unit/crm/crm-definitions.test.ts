import { describe, expect, test } from 'bun:test';
import {
	appendCRMDefinition,
	cloneCRMVocabulary,
	removeCRMDefinition,
	updateCRMDefinition,
	type CRMVocabulary
} from '../../../src/routes/crm/crm-definitions';

const vocabulary: CRMVocabulary = {
	organization_types: [{ id: 'partner', name: '파트너', color: '#111111' }],
	pipelines: [{ id: 'sponsorship', name: '후원' }]
};

describe('CRM definitions', () => {
	test('clones definitions without sharing mutable arrays', () => {
		const clone = cloneCRMVocabulary(vocabulary);
		clone.organization_types.push({ id: 'vendor', name: '협력사' });
		expect(vocabulary.organization_types).toHaveLength(1);
	});

	test('adds a definition to the requested collection', () => {
		const withPipeline = appendCRMDefinition(vocabulary, 'pipeline', { id: 'sales', name: '판매' });
		expect(withPipeline.pipelines[1]).toEqual({ id: 'sales', name: '판매' });
	});

	test('updates a definition without changing its stable identifier', () => {
		const next = updateCRMDefinition(
			vocabulary,
			{ kind: 'organization_type', id: 'partner' },
			{ name: '핵심 파트너', color: '#222222' }
		);
		expect(next.organization_types[0]).toEqual({
			id: 'partner',
			name: '핵심 파트너',
			color: '#222222'
		});
	});

	test('removes only the requested definition and leaves the input unchanged', () => {
		const next = removeCRMDefinition(vocabulary, { kind: 'pipeline', id: 'sponsorship' });
		expect(next.pipelines).toEqual([]);
		expect(vocabulary.pipelines).toHaveLength(1);
	});
});
