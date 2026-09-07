<script lang="ts">
	import FilterCombobox, { type FilterComboboxOption } from '$lib/components/filter-combobox.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import type { CRMText } from './text';

	type OwnerOption = FilterComboboxOption & {
		email: string;
		image: string;
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
		value: person.memberID,
		label: displayPersonName(person.name || person.email),
		keywords: [person.name ?? '', person.email, person.handle, groupName(person.groupID)],
		email: person.email,
		image: person.image ?? ''
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
			<PersonAvatar name={option.label} email={option.email} seed={option.value} image={option.image} class="size-4 shrink-0" /><span class="truncate">{option.label}</span>
			<span class="truncate text-xs text-muted-foreground">{option.email}</span>
		</div>
	{/snippet}
	{#snippet selectedContent(option)}
		<PersonChip name={option.label} email={option.email} seed={option.value} image={option.image} />
	{/snippet}
</FilterCombobox>
