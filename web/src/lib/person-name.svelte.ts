import { currentLocale } from './i18n/locale.svelte';
import { personName, personNameLocale } from './person-name';

let companyLocale = $state('');

export function setPersonNameCompanyLocale(locale: string): void {
	companyLocale = locale;
}

export function displayPersonName(recorded: string | null | undefined): string {
	return personName(recorded ?? '', currentPersonNameLocale());
}

export function currentPersonNameLocale() {
	return personNameLocale(currentLocale.value, companyLocale);
}
