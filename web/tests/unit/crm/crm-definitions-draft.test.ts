import { afterAll, describe, expect, test } from 'bun:test';
import { appendCRMDefinition } from '../../../src/routes/crm/crm-definitions';
import type { CRMVocabulary } from '../../../src/routes/crm/crm-api-types';

const originalState = Reflect.get(globalThis, '$state');
Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const { CRMDefinitionsDraft } = await import('../../../src/routes/crm/crm-definitions-draft.svelte');

afterAll(() => {
	Reflect.set(globalThis, '$state', originalState);
});

const blankNameMessage = '이름을 입력하세요.';

function vocabularyOf(names: string[]): CRMVocabulary {
	return {
		organization_types: names.map((name, index) => ({ id: `type-${index}`, name })),
		pipelines: []
	};
}

function organizationTypeNames(vocabulary: CRMVocabulary): string[] {
	return vocabulary.organization_types.map((definition) => definition.name);
}

function withOrganizationType(vocabulary: CRMVocabulary, name: string): CRMVocabulary {
	return appendCRMDefinition(vocabulary, 'organization_type', { id: name, name });
}

function settle(): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('CRM definitions draft', () => {
	test('shows an added definition before its save answers', async () => {
		const remote = vocabularyOf(['하나']);
		let releaseSave = (): void => {};
		const heldSave = new Promise<void>((resolve) => {
			releaseSave = resolve;
		});
		const draft = new CRMDefinitionsDraft(
			() => remote,
			() => heldSave,
			() => blankNameMessage
		);
		draft.synchronize();

		draft.commit(withOrganizationType(draft.value, '둘'));

		expect(organizationTypeNames(draft.value)).toEqual(['하나', '둘']);
		expect(draft.isSaving).toBe(true);

		releaseSave();
		await settle();
		expect(draft.isSaving).toBe(false);
	});

	test('keeps an edit made while an earlier save is still running', async () => {
		const remote = vocabularyOf(['하나']);
		const saved: CRMVocabulary[] = [];
		let releaseSave = (): void => {};
		const heldSave = new Promise<void>((resolve) => {
			releaseSave = resolve;
		});
		const draft = new CRMDefinitionsDraft(
			() => remote,
			(vocabulary) => {
				saved.push(vocabulary);
				return heldSave;
			},
			() => blankNameMessage
		);
		draft.synchronize();

		draft.commit(withOrganizationType(draft.value, '둘'));
		draft.commit(withOrganizationType(draft.value, '셋'));
		expect(organizationTypeNames(draft.value)).toEqual(['하나', '둘', '셋']);

		releaseSave();
		await settle();

		expect(saved).toHaveLength(2);
		expect(organizationTypeNames(saved[1])).toEqual(['하나', '둘', '셋']);
	});

	test('rolls back to the stored vocabulary when a save fails', async () => {
		const remote = vocabularyOf(['하나']);
		const draft = new CRMDefinitionsDraft(
			() => remote,
			() => {
				throw new Error('등록된 CRM 기록에서 사용 중인 항목입니다.');
			},
			() => blankNameMessage
		);
		draft.synchronize();

		draft.commit(withOrganizationType(draft.value, '둘'));
		await settle();

		expect(organizationTypeNames(draft.value)).toEqual(['하나']);
		expect(draft.errorMessage).toBe('등록된 CRM 기록에서 사용 중인 항목입니다.');
	});

	test('refuses a blank name without sending a save', async () => {
		const remote = vocabularyOf(['하나']);
		const saved: CRMVocabulary[] = [];
		const draft = new CRMDefinitionsDraft(
			() => remote,
			(vocabulary) => {
				saved.push(vocabulary);
			},
			() => blankNameMessage
		);
		draft.synchronize();

		draft.commit(withOrganizationType(draft.value, '  '));
		await settle();

		expect(saved).toHaveLength(0);
		expect(draft.errorMessage).toBe(blankNameMessage);
		expect(organizationTypeNames(draft.value)).toEqual(['하나']);
	});

	test('does not overwrite an unsaved edit when the stored vocabulary arrives', async () => {
		const remote = vocabularyOf(['하나']);
		let releaseSave = (): void => {};
		const heldSave = new Promise<void>((resolve) => {
			releaseSave = resolve;
		});
		const draft = new CRMDefinitionsDraft(
			() => remote,
			() => heldSave,
			() => blankNameMessage
		);
		draft.synchronize();

		draft.commit(withOrganizationType(draft.value, '둘'));
		draft.synchronize();

		expect(organizationTypeNames(draft.value)).toEqual(['하나', '둘']);

		releaseSave();
		await settle();
	});
});
