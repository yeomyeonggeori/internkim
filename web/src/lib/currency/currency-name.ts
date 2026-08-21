import type { CurrencyCatalogueEntry } from './currency-catalogue';
import type { Locale } from '$lib/i18n/locale.svelte';

export function currencyDisplayNamesFor(locale: Locale): Intl.DisplayNames {
	return new Intl.DisplayNames([locale === 'ko' ? 'ko' : 'en'], { type: 'currency', fallback: 'none' });
}

export function currencyNameOf(entry: CurrencyCatalogueEntry, displayNames: Intl.DisplayNames): string {
	try {
		return displayNames.of(entry.code) ?? entry.name;
	} catch {
		return entry.name;
	}
}
