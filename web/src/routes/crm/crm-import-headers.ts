import type { CRMRecordImportKind } from '$lib/crm/crm-record-import';
import type { CRMImportField } from './crm-import-mapping';
import { crmImportText } from './crm-import-text';
import { crmText } from './text';

type CRMMessages = (typeof crmText)[keyof typeof crmText];
type ExportedHeader = (messages: CRMMessages) => string;

const exportedHeaders: Record<CRMRecordImportKind, Partial<Record<CRMImportField, ExportedHeader>>> = {
	organization: {
		name: (messages) => messages.name,
		types: (messages) => messages.type,
		status: (messages) => messages.status,
		importance: (messages) => messages.importance,
		ownerPersonHint: (messages) => messages.internalOwner,
		address: (messages) => messages.address,
		tags: (messages) => messages.tags,
		description: (messages) => messages.details
	},
	contact: {
		name: (messages) => messages.contactName,
		organizationName: (messages) => messages.organizationName,
		role: (messages) => messages.contactTitle,
		department: (messages) => messages.export.department,
		email: (messages) => messages.email,
		phoneNumber: (messages) => messages.phone,
		description: (messages) => messages.details
	}
};

export function crmImportFieldNames(kind: CRMRecordImportKind): (field: CRMImportField) => string[] {
	return (field) => {
		const exported = exportedHeaders[kind][field];
		return [
			field,
			crmImportText.ko.fields[field],
			crmImportText.en.fields[field],
			...(exported ? [exported(crmText.ko), exported(crmText.en)] : [])
		];
	};
}
