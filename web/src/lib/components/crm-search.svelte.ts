import { interimCurrencyCatalogue } from '$lib/currency/currency-catalogue';
import { matchesKoreanSearch } from '$lib/korean-search';
import { loadCRMData } from '../../routes/crm/crm-data-source';
import { mapCRMViewData } from '../../routes/crm/crm-mappers';
import type { CRMContact, CRMOrganization } from '../../routes/crm/crm-types';

const searchResultLimit = 5;

export type CRMSearchResults = {
	organizations: CRMOrganization[];
	contacts: CRMContact[];
};

function organizationMatchesQuery(organization: CRMOrganization, query: string): boolean {
	return [organization.name, ...organization.tags].some((value) => matchesKoreanSearch(value, query));
}

function contactMatchesQuery(contact: CRMContact, query: string): boolean {
	return [contact.name, contact.email, contact.title].some((value) => matchesKoreanSearch(value, query));
}

class CRMSearch {
	organizations = $state<CRMOrganization[]>([]);
	contacts = $state<CRMContact[]>([]);

	load = async () => {
		try {
			const view = mapCRMViewData(await loadCRMData(), [], interimCurrencyCatalogue);
			this.organizations = view.organizations;
			this.contacts = view.contacts;
		} catch {
			this.organizations = [];
			this.contacts = [];
		}
	};

	organizationNameOf = (organizationID: string): string =>
		this.organizations.find((organization) => organization.id === organizationID)?.name ?? '';

	search = (query: string): CRMSearchResults => {
		const trimmedQuery = query.trim();
		if (!trimmedQuery) return { organizations: [], contacts: [] };
		return {
			organizations: this.organizations
				.filter((organization) => organizationMatchesQuery(organization, trimmedQuery))
				.slice(0, searchResultLimit),
			contacts: this.contacts.filter((contact) => contactMatchesQuery(contact, trimmedQuery)).slice(0, searchResultLimit)
		};
	};
}

export const crmSearch = new CRMSearch();
