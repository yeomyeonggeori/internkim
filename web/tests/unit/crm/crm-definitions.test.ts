import { describe, expect, test } from 'bun:test';
import {
	appendCRMDefinition,
	appendCRMStage,
	cloneCRMVocabulary,
	moveCRMDefinition,
	removeCRMDefinition,
	updateCRMDefinition,
	type CRMVocabulary
} from '../../../src/routes/crm/crm-definitions';

const vocabulary: CRMVocabulary = {
	organization_types: [{ id: 'partner', name: '파트너', color: '#111111' }],
	pipelines: [{
		id: 'sponsorship',
		name: '후원',
		stages: [
			{ id: 'review', name: '검토', outcome: 'open' },
			{ id: 'won', name: '성사', outcome: 'won' }
		]
	}],
	lost_reasons: [{ id: 'budget', name: '예산 부족' }]
};

describe('CRM definitions', () => {
	test('clones nested stages without sharing mutable arrays', () => {
		const clone = cloneCRMVocabulary(vocabulary);
		clone.pipelines[0]?.stages.push({ id: 'lost', name: '불발', outcome: 'lost' });
		expect(vocabulary.pipelines[0]?.stages).toHaveLength(2);
	});

	test('adds top-level definitions and pipeline stages', () => {
		const withPipeline = appendCRMDefinition(vocabulary, 'pipeline', { id: 'sales', name: '판매' });
		const withStage = appendCRMStage(withPipeline, 'sales', { id: 'proposal', name: '제안', outcome: 'open' });
		expect(withStage.pipelines[1]).toEqual({
			id: 'sales',
			name: '판매',
			stages: [{ id: 'proposal', name: '제안', outcome: 'open' }]
		});
	});

	test('updates a nested stage without changing its stable identifier', () => {
		const next = updateCRMDefinition(
			vocabulary,
			{ kind: 'stage', pipelineID: 'sponsorship', id: 'review' },
			{ name: '협의', color: '#222222', outcome: 'on_hold' }
		);
		expect(next.pipelines[0]?.stages[0]).toEqual({
			id: 'review',
			name: '협의',
			color: '#222222',
			outcome: 'on_hold'
		});
	});

	test('reorders entries within their own collection', () => {
		const next = moveCRMDefinition(
			vocabulary,
			{ kind: 'stage', pipelineID: 'sponsorship', id: 'won' },
			-1
		);
		expect(next.pipelines[0]?.stages.map((stage) => stage.id)).toEqual(['won', 'review']);
	});

	test('removes only the requested definition and leaves the input unchanged', () => {
		const next = removeCRMDefinition(vocabulary, { kind: 'lost_reason', id: 'budget' });
		expect(next.lost_reasons).toEqual([]);
		expect(vocabulary.lost_reasons).toHaveLength(1);
	});
});
