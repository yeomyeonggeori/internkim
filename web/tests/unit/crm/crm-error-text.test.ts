import { describe, expect, test } from 'bun:test';
import { CRMApiError } from '../../../src/routes/crm/crm-error';
import { crmErrorMessage, CRMPageError } from '../../../src/routes/crm/crm-error-text';
import { CRMOwnerResolutionError } from '../../../src/routes/crm/crm-mappers';
import { CRMOpportunityTransitionError } from '../../../src/routes/crm/crm-opportunity-transition';
import { crmText } from '../../../src/routes/crm/text';

describe('CRM localized errors', () => {
	test('localizes permission errors for Korean and English', () => {
		const error = new CRMApiError('raw service message', 403, 'permission_denied');

		expect(crmErrorMessage(error, crmText.ko)).toBe('CRM을 보거나 수정할 권한이 없습니다.');
		expect(crmErrorMessage(error, crmText.en)).toBe('You do not have permission to view or edit CRM records.');
	});

	test('localizes owner resolution errors for Korean and English', () => {
		const error = new CRMOwnerResolutionError('owner_ambiguous');

		expect(crmErrorMessage(error, crmText.ko)).toBe('담당자가 여러 명입니다. 이메일을 입력해 주세요.');
		expect(crmErrorMessage(error, crmText.en)).toBe('Multiple owners match. Enter an email address.');
	});

	test('localizes organization directory errors after locale initialization', () => {
		const error = new CRMPageError('organization_load_failed');

		expect(crmErrorMessage(error, crmText.ko)).toBe('조직도 정보를 불러오지 못했습니다.');
		expect(crmErrorMessage(error, crmText.en)).toBe('Could not load the organization directory.');
	});

	test('distinguishes a saved record refresh failure from a failed save', () => {
		const error = new CRMPageError('refresh_after_save_failed');

		expect(crmErrorMessage(error, crmText.ko)).toBe('저장했지만 목록을 새로 불러오지 못했습니다. 다시 시도해 주세요.');
		expect(crmErrorMessage(error, crmText.en)).toBe('Saved, but the list could not be refreshed. Try again.');
	});

	test('localizes the required lost reason', () => {
		const error = new CRMOpportunityTransitionError('lost_reason_required');

		expect(crmErrorMessage(error, crmText.ko)).toBe('실패 단계를 선택할 때는 실패 사유가 필요합니다.');
		expect(crmErrorMessage(error, crmText.en)).toBe('Select a lost reason before moving to a lost stage.');
	});

});
