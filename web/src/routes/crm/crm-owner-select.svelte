<script lang="ts">
	import FilterCombobox, { type FilterComboboxOption } from '$lib/components/filter-combobox.svelte';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import type { CRMText } from './text';

	type OwnerOption = FilterComboboxOption & {
		email: string;
		team: string;
	};

	type Props = {
		value: string;
		people: UserRecord[];
		groups: OrgGroup[];
		text: CRMText;
		id?: string;
		disabled?: boolean;
	};

	let { value = $bindable(''), people, groups, text, id, disabled = false }: Props = $props();
	let options = $derived<OwnerOption[]>(people.map((person) => ({
		value: person.userID,
		label: person.name || person.email,
		keywords: [person.email, person.handle, groupName(person.groupID)],
		email: person.email,
		team: groupName(person.groupID)
	})));

	function groupName(groupID: string | undefined): string {
		return groups.find((group) => group.id === groupID)?.name ?? '';
	}
</script>

<FilterCombobox
	bind:value
	{options}
	label={text.selectInternalOwner}
	searchPlaceholder={text.searchInternalOwner}
	canClearSelection={false}
	class="w-full"
	contentClass="w-(--bits-popover-anchor-width)"
	{id}
	{disabled}
>
	{#snippet optionContent(option)}
		<div class="flex min-w-0 items-center gap-2 text-left">
			<span class="shrink-0">{option.label}</span>
			<span class="truncate text-xs text-muted-foreground">{[option.email, option.team].filter(Boolean).join(' · ')}</span>
		</div>
	{/snippet}
	{#snippet selectedContent(option)}
		<span class="truncate">{option.label}</span>
	{/snippet}
</FilterCombobox>
