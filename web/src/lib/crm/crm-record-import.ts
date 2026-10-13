export const crmRecordImportKinds = ['organization', 'contact'] as const;
export const crmRecordImportDuplicateChoices = ['skip', 'update'] as const;
export const crmRecordImportOutcomes = ['create', 'update', 'skip', 'refused'] as const;
export const crmRecordImportProblems = [
	'name_missing',
	'name_repeated_in_file',
	'organization_unknown',
	'organization_ambiguous',
	'type_unknown',
	'status_unknown',
	'importance_unknown',
	'owner_unresolved',
	'email_invalid',
	'contact_unreachable',
	'field_of_other_kind'
] as const;

export type CRMRecordImportKind = (typeof crmRecordImportKinds)[number];
export type CRMRecordImportDuplicateChoice = (typeof crmRecordImportDuplicateChoices)[number];
export type CRMRecordImportOutcome = (typeof crmRecordImportOutcomes)[number];
export type CRMRecordImportProblem = (typeof crmRecordImportProblems)[number];
