import type { OrgGroup, UserRecord } from '$lib/organization/types';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { CRMDataResponse } from './crm-api-types';

export type CRMDataSnapshot = {
	email: string;
	data: CRMDataResponse;
	currencyCatalogue: CurrencyCatalogue;
	companyBaseCurrency: string;
	people: UserRecord[];
	groups: OrgGroup[];
	isDirectoryReady: boolean;
};

let lastSnapshot: CRMDataSnapshot | undefined;

export function rememberedCRMData(email: string): CRMDataSnapshot | undefined {
	return lastSnapshot?.email === email ? lastSnapshot : undefined;
}

export function rememberCRMData(snapshot: CRMDataSnapshot): void {
	lastSnapshot = snapshot;
}

export function forgetCRMData(): void {
	lastSnapshot = undefined;
}
