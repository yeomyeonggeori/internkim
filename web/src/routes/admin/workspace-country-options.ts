import type { HolidayCountry } from './admin-types';

export type WorkspaceCountryOption = {
	value: string;
	label: string;
};

export function workspaceCountryOptions(
	countries: HolidayCountry[],
	locale: string,
	selectedCountryCode = ''
): WorkspaceCountryOption[] {
	const displayNames = new Intl.DisplayNames([locale], { type: 'region' });
	const normalizedSelectedCountryCode = selectedCountryCode.trim().toUpperCase();
	const optionCountries =
		normalizedSelectedCountryCode && !countries.some((country) => country.countryCode === normalizedSelectedCountryCode)
			? [...countries, { countryCode: normalizedSelectedCountryCode, name: normalizedSelectedCountryCode }]
			: countries;
	const options = optionCountries.map((country) => {
		const countryName = displayNames.of(country.countryCode);
		return {
			value: country.countryCode,
			label: `${countryName && countryName !== country.countryCode ? countryName : country.name} (${country.countryCode})`
		};
	});
	return options.sort((first, second) => first.label.localeCompare(second.label, locale));
}
