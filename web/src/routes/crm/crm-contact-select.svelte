<script lang="ts">
	import FilterCombobox, { type FilterComboboxOption } from '$lib/components/filter-combobox.svelte';
	import type { CRMContact } from './crm-types';
	import type { CRMText } from './text';

	type ContactOption = FilterComboboxOption & {
		detail: string;
	};

	type Props = {
		value: string;
		contacts: CRMContact[];
		text: CRMText;
		id?: string;
		disabled?: boolean;
	};

	let { value = $bindable(''), contacts, text, id, disabled = false }: Props = $props();
	const noContactValue = '__no_contact__';
	let options = $derived<ContactOption[]>([
		{ value: noContactValue, label: text.none, detail: '' },
		...contacts.map((contact) => ({
			value: contact.id,
			label: contact.name,
			keywords: [contact.email ?? '', contact.phone ?? '', contact.title ?? ''],
			detail: [contact.title, contact.email || contact.phone].filter(Boolean).join(' · ')
		}))
	]);
	let selectedValue = $derived(value || noContactValue);

	function selectContact(nextValue: string): void {
		value = nextValue === noContactValue ? '' : nextValue;
	}
</script>

<FilterCombobox
	value={selectedValue}
	{options}
	label={text.selectExternalContact}
	searchPlaceholder={text.searchExternalContact}
	canClearSelection={false}
	onSelect={selectContact}
	class="w-full"
	contentClass="w-(--bits-popover-anchor-width)"
	{id}
	{disabled}
>
	{#snippet optionContent(option)}
		<div class="flex min-w-0 items-center gap-2 text-left">
			<span class="shrink-0">{option.label}</span>
			{#if option.detail}<span class="truncate text-xs text-muted-foreground">{option.detail}</span>{/if}
		</div>
	{/snippet}
	{#snippet selectedContent(option)}
		<span class="truncate">{option.label}</span>
	{/snippet}
</FilterCombobox>
