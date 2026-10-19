import { describe, expect, test } from 'bun:test';
import { crmExportTable } from '../../src/routes/crm/crm-export-table';
import { crmImportFieldNames } from '../../src/routes/crm/crm-import-headers';
import { crmText, type CRMText } from '../../src/routes/crm/text';
import {
	crmImportFields,
	importedRows,
	missingRequiredFields,
	suggestedMapping,
	withField,
	type CRMImportField
} from '../../src/lib/crm/crm-import-mapping';

const koreanNames: Partial<Record<CRMImportField, string>> = { name: '이름', address: '주소' };

function fieldNames(field: CRMImportField): string[] {
	const korean = koreanNames[field];
	return korean ? [field, korean] : [field];
}

const labels = { status: [{ prospect: '잠재', active: '활성', paused: '중단' }], importance: [] };

describe('columns of an imported file', () => {
	test('are matched only to a field whose name they carry exactly', () => {
		expect(
			suggestedMapping([' 이름 ', '회사명', 'ADDRESS', '이름'], crmImportFields.organization, fieldNames)
		).toEqual(['name', null, 'address', null]);
	});

	test('give a field to one column at a time', () => {
		expect(withField(['name', null, 'address'], 1, 'name')).toEqual([null, 'name', 'address']);
		expect(withField(['name', null], 0, null)).toEqual([null, null]);
	});

	test('cannot go on without the fields a record needs', () => {
		expect(missingRequiredFields('contact', ['name', null])).toEqual(['organizationName']);
		expect(missingRequiredFields('organization', ['name'])).toEqual([]);
	});

	test('become rows that keep their line, split lists and read a status by its label', () => {
		const rows = importedRows(
			['name', 'tags', 'status', null],
			[{ line: 3, cells: ['샘플상사', '제조, 수출;우수', '활성', '버릴 값'] }],
			labels
		);

		expect(rows).toEqual([{ line: 3, name: '샘플상사', tags: ['제조', '수출', '우수'], status: 'active' }]);
	});
});

describe('a file the CRM exported', () => {
	function exportedColumns(tab: 'relationships' | 'contacts', text: CRMText): string[] {
		return crmExportTable(tab, {
			organizations: [],
			contacts: [],
			opportunities: [],
			activities: [],
			organizationTypeDefinitions: [],
			pipelines: [],
			stages: [],
			companyBaseCurrency: 'KRW',
			timeZone: 'Asia/Seoul',
			text
		}).columns;
	}

	for (const text of [crmText.ko, crmText.en]) {
		test(`matches every column it can import by itself in ${text === crmText.ko ? 'Korean' : 'English'}`, () => {
			const organizationColumns = exportedColumns('relationships', text);
			expect(
				suggestedMapping(organizationColumns, crmImportFields.organization, crmImportFieldNames('organization'))
			).toEqual(['name', 'types', 'status', 'importance', 'ownerPersonHint', null, 'address', 'tags', 'description']);

			const contactColumns = exportedColumns('contacts', text);
			expect(suggestedMapping(contactColumns, crmImportFields.contact, crmImportFieldNames('contact'))).toEqual([
				'name',
				'organizationName',
				'role',
				'department',
				'email',
				'phoneNumber',
				'description'
			]);
		});
	}
});
