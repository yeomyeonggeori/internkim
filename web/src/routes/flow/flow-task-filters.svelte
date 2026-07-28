<script lang="ts">
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Input } from '$lib/components/ui/input';
	import FilterIcon from '@lucide/svelte/icons/sliders-horizontal';
	import PlusIcon from '@lucide/svelte/icons/plus';
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
		hasMembers: boolean;
		text: FlowPageText['filters'];
		resetFilters: () => void;
		createTask: () => void;
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
		hasMembers,
		text,
		resetFilters,
		createTask,
		setParticipantFilterIDs,
		class: className
	}: Props = $props();

	let participantChoices = $derived(participantOptions.filter((option) => option.value !== 'all'));
	let participantChoiceIDs = $derived(participantChoices.map((option) => option.value));
	let activeFilterCount = $derived(
		(searchText.trim() ? 1 : 0) +
		(statusFilter !== 'all' ? 1 : 0) +
		(participantFilterIDs.length > 0 ? 1 : 0) +
		(businessFilter !== 'all' ? 1 : 0) +
		(typeFilter !== 'all' ? 1 : 0)
	);
	let filterButtonLabel = $derived(activeFilterCount > 0 ? text.filterCount.replace('{count}', String(activeFilterCount)) : text.filterButton);

	function toggleParticipant(memberID: string): void {
		if (participantFilterIDs.length === 0) {
			setParticipantFilterIDs(participantChoiceIDs.filter((value) => value !== memberID));
			return;
		}
		const nextParticipantFilterIDs = participantFilterIDs.includes(memberID)
			? participantFilterIDs.filter((value) => value !== memberID)
			: [...participantFilterIDs, memberID];
		const hasSelectedEveryParticipant = participantChoiceIDs.every((value) => nextParticipantFilterIDs.includes(value));
		setParticipantFilterIDs(hasSelectedEveryParticipant ? [] : nextParticipantFilterIDs);
	}
</script>

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
			</div>
			<div class="space-y-2">
				<div class="flex items-center justify-between gap-2">
					<div class="text-xs font-medium text-muted-foreground">{text.participants}</div>
					<Button type="button" variant="ghost" size="sm" class="h-7 px-2" onclick={() => setParticipantFilterIDs([])}>
						{text.allParticipants}
					</Button>
				</div>
				<div class="grid max-h-56 gap-1 overflow-y-auto pr-1">
					{#each participantChoices as option (option.value)}
						<label class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-muted">
							<input
								type="checkbox"
								class="size-4 accent-primary"
								checked={participantFilterIDs.length === 0 || participantFilterIDs.includes(option.value)}
								onchange={() => toggleParticipant(option.value)}
							/>
							<PersonAvatar name={option.label} email={option.email ?? ''} seed={option.value} image={option.image ?? ''} class="size-6" />
							<span class="min-w-0 truncate">{option.label}</span>
						</label>
					{/each}
				</div>
			</div>
			<div class="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
				<Button type="button" variant="ghost" size="sm" onclick={resetFilters}>
					<RotateCcwIcon class="size-4" />
					{text.reset}
				</Button>
				<Button type="button" size="sm" onclick={() => createTask()} disabled={!hasMembers}>
					<PlusIcon class="size-4" />
					{text.addTask}
				</Button>
			</div>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>
