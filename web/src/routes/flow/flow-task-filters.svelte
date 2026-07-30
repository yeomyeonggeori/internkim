<script lang="ts">
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Input } from '$lib/components/ui/input';
	import FilterIcon from '@lucide/svelte/icons/sliders-horizontal';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { cn } from '$lib/utils';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Option = {
		value: string;
		label: string;
		email?: string;
		image?: string;
	};

	type Props = {
		searchText: string;
		statusFilter: string;
		businessFilter: string;
		typeFilter: string;
		participantFilterIDs: string[];
		statusOptions: Option[];
		participantOptions: Option[];
		businessOptions: Option[];
		typeOptions: Option[];
		hasBusinessFilter: boolean;
		text: FlowPageText['filters'];
		resetFilters: () => void;
		setParticipantFilterIDs: (memberIDs: string[]) => void;
		class?: string;
	};

	let {
		searchText = $bindable(''),
		statusFilter = $bindable('all'),
		businessFilter = $bindable('all'),
		typeFilter = $bindable('all'),
		participantFilterIDs,
		statusOptions,
		participantOptions,
		businessOptions,
		typeOptions,
		hasBusinessFilter,
		text,
		resetFilters,
		setParticipantFilterIDs,
		class: className
	}: Props = $props();

	let participantValue = $state('all');
	let activeFilterCount = $derived(
		(searchText.trim() ? 1 : 0) +
		(statusFilter !== 'all' ? 1 : 0) +
		(participantFilterIDs.length > 0 ? 1 : 0) +
		(businessFilter !== 'all' ? 1 : 0) +
		(typeFilter !== 'all' ? 1 : 0)
	);
	let filterButtonLabel = $derived(activeFilterCount > 0 ? text.filterCount.replace('{count}', String(activeFilterCount)) : text.filterButton);

	$effect(() => {
		participantValue = participantFilterIDs[0] ?? 'all';
	});

	function selectParticipant(memberID: string): void {
		setParticipantFilterIDs(memberID === 'all' ? [] : [memberID]);
	}
</script>

{#snippet participantOption(option: Option)}
	{#if option.value === 'all'}
		<span class="min-w-0 truncate">{option.label}</span>
	{:else}
		<PersonAvatar name={option.label} email={option.email ?? ''} seed={option.value} image={option.image ?? ''} class="size-5" />
		<span class="min-w-0 truncate">{option.label}</span>
	{/if}
{/snippet}

<div class={cn('flex min-w-0 flex-wrap items-center gap-2', className)}>
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} type="button" variant="outline" size="sm" class="gap-2">
					<FilterIcon class="size-4" />
					{filterButtonLabel}
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" sideOffset={8} class="w-[min(24rem,calc(100vw-2rem))] space-y-4 p-3" data-flow-filter-panel>
			<div class="relative">
				<SearchIcon class="absolute left-2 top-2.5 size-4 text-muted-foreground" />
				<Input class="pl-8" placeholder={text.searchPlaceholder} bind:value={searchText} />
			</div>
			<div class="flex flex-wrap gap-2">
				<FilterCombobox bind:value={statusFilter} options={statusOptions} label={text.status} clearValue="all" class="w-full sm:w-[calc(50%-0.25rem)]" />
				<FilterCombobox bind:value={typeFilter} options={typeOptions} label={text.type} clearValue="all" class="w-full sm:w-[calc(50%-0.25rem)]" />
				{#if hasBusinessFilter}
					<FilterCombobox bind:value={businessFilter} options={businessOptions} label={text.business} clearValue="all" class="w-full sm:w-[calc(50%-0.25rem)]" />
				{/if}
				<FilterCombobox
					bind:value={participantValue}
					options={participantOptions}
					label={text.participants}
					clearValue="all"
					onSelect={selectParticipant}
					class="w-full sm:w-[calc(50%-0.25rem)]"
					optionContent={participantOption}
				/>
			</div>
			<div class="flex items-center justify-end border-t pt-3">
				<Button type="button" variant="ghost" size="sm" onclick={resetFilters}>
					<RotateCcwIcon class="size-4" />
					{text.reset}
				</Button>
			</div>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>
