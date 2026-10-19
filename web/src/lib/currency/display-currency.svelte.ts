import { browser } from '$app/environment';

const storageKey = 'internkim.display-currency';
const euroRegions = ['AT', 'BE', 'CY', 'DE', 'EE', 'ES', 'FI', 'FR', 'GR', 'HR', 'IE', 'IT', 'LT', 'LU', 'LV', 'MT', 'NL', 'PT', 'SI', 'SK'];
const currencyByRegion = new Map<string, string>([
	['KR', 'KRW'],
	['US', 'USD'],
	['JP', 'JPY'],
	...euroRegions.map((region) => [region, 'EUR'] as const)
]);

let displayCurrencyValue = $state(browser ? (localStorage.getItem(storageKey) ?? currencyOfBrowserRegion()) : '');

export const displayCurrency = {
	get value() {
		return displayCurrencyValue;
	}
};

export function setDisplayCurrency(currency: string) {
	displayCurrencyValue = currency;
	if (browser) localStorage.setItem(storageKey, currency);
}

function currencyOfBrowserRegion(): string {
	const region = new Intl.Locale(navigator.language).maximize().region ?? '';
	return currencyByRegion.get(region) ?? '';
}
