import { describe, expect, test } from 'bun:test';
import {
	appendCRMDefinition,
	appendCRMStage,
	cloneCRMVocabulary,
	removeCRMDefinition,
	updateCRMDefinition,
	type CRMVocabulary
} from '../../../src/routes/crm/crm-definitions';

const vocabulary: CRMVocabulary = {
	organization_types: [{ id: 'partner', name: '파트너', color: '#111111' }],
	pipelines: [{ id: 'sponsorship', name: '후원' }],
	stages: [
		{ id: 'review', name: '검토', outcome: 'open' },
		{ id: 'won', name: '성사', outcome: 'won' }
	],
	lost_reasons: [{ id: 'budget', name: '예산 부족' }]
};

describe('CRM definitions', () => {
	test('clones stages without sharing mutable arrays', () => {
		const clone = cloneCRMVocabulary(vocabulary);
		clone.stages.push({ id: 'lost', name: '불발', outcome: 'lost' });
		expect(vocabulary.stages).toHaveLength(2);
	});

	test('adds top-level definitions and global stages', () => {
		const withPipeline = appendCRMDefinition(vocabulary, 'pipeline', { id: 'sales', name: '판매' });
		const withStage = appendCRMStage(withPipeline, { id: 'proposal', name: '제안', outcome: 'open' });
		expect(withStage.pipelines[1]).toEqual({ id: 'sales', name: '판매' });
		expect(withStage.stages[2]).toEqual({ id: 'proposal', name: '제안', outcome: 'open' });
	});

	test('updates a stage without changing its stable identifier', () => {
		const next = updateCRMDefinition(
			vocabulary,
			{ kind: 'stage', id: 'review' },
			{ name: '협의', color: '#222222', outcome: 'on_hold' }
		);
		expect(next.stages[0]).toEqual({
			id: 'review',
			name: '협의',
			color: '#222222',
			outcome: 'on_hold'
		});
	});

	test('removes only the requested definition and leaves the input unchanged', () => {
		const next = removeCRMDefinition(vocabulary, { kind: 'lost_reason', id: 'budget' });
		expect(next.lost_reasons).toEqual([]);
		expect(vocabulary.lost_reasons).toHaveLength(1);
	});
});
