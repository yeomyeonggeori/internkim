<script lang="ts">
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import type { HolidayCountry } from './admin-types';
	import { workspaceCountryOptions } from './workspace-country-options';

	type WorkspaceCountrySelectProps = {
		countryCode: string;
		disabled: boolean;
		label: string;
		locale: string;
		countries: HolidayCountry[];
	};

	let { countryCode = $bindable(), disabled, label, locale, countries }: WorkspaceCountrySelectProps = $props();
	const options = $derived(workspaceCountryOptions(countries, locale));
</script>

<FilterCombobox
	bind:value={countryCode}
	{options}
	{label}
	{disabled}
	id="workspace-country-code"
	canClearSelection={false}
	class="w-full"
/>
