import type { UserRecord } from '$lib/organization/types';
import { createCRMOrganization, createCRMContact } from './crm-data-source';
import { organizationPayloadFromDraft, contactPayloadFromDraft } from './crm-mappers';
import type { CRMRelationshipCreateDraft } from './crm-types';

export class CRMRelationshipContactCreateError extends Error {
	constructor(readonly organizationID: string, message: string) {
		super(message);
		this.name = 'CRMRelationshipContactCreateError';
	}
}

export async function createCRMRelationshipRecords(
	draft: CRMRelationshipCreateDraft,
	owner: UserRecord,
	contactFailureMessage: string
): Promise<void> {
	const organizationID = draft.createdOrganizationID
		?? (await createCRMOrganization(organizationPayloadFromDraft(draft, owner))).id;
	if (!draft.contact) return;
	try {
		await createCRMContact(contactPayloadFromDraft({
			kind: 'contact',
			organizationID,
			...draft.contact
		}, owner));
	} catch {
		throw new CRMRelationshipContactCreateError(organizationID, contactFailureMessage);
	}
}
