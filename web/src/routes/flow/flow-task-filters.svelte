<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import FilterIcon from '@lucide/svelte/icons/sliders-horizontal';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import SearchIcon from '@lucide/svelte/icons/search';
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
		setParticipantFilterIDs
	}: Props = $props();

	let participantChoices = $derived(participantOptions.filter((option) => option.value !== 'all'));
	let activeFilterCount = $derived(
		(searchText.trim() ? 1 : 0) +
		(statusFilter !== 'all' ? 1 : 0) +
		(participantFilterIDs.length > 0 ? 1 : 0) +
		(businessFilter !== 'all' ? 1 : 0) +
		(typeFilter !== 'all' ? 1 : 0)
	);
	let filterButtonLabel = $derived(activeFilterCount > 0 ? text.filterCount.replace('{count}', String(activeFilterCount)) : text.filterButton);

	function toggleParticipant(memberID: string): void {
		if (participantFilterIDs.includes(memberID)) {
			setParticipantFilterIDs(participantFilterIDs.filter((value) => value !== memberID));
			return;
		}
		setParticipantFilterIDs([...participantFilterIDs, memberID]);
	}
</script>

<div class="flex min-w-0 flex-wrap items-center gap-2">
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
			<div class="grid gap-3 sm:grid-cols-2">
				{@render SelectControl(text.status, statusOptions, statusFilter, (value: string) => (statusFilter = value))}
				{@render SelectControl(text.type, typeOptions, typeFilter, (value: string) => (typeFilter = value))}
				{#if hasBusinessFilter}
					{@render SelectControl(text.business, businessOptions, businessFilter, (value: string) => (businessFilter = value))}
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
								checked={participantFilterIDs.includes(option.value)}
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

{#snippet SelectControl(label: string, options: Option[], value: string, onchange: (value: string) => void)}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{label}
		<Select.Root type="single" {value} onValueChange={onchange}>
			<Select.Trigger class="w-full">
				{options.find((option) => option.value === value)?.label ?? '-'}
			</Select.Trigger>
			<Select.Content>
				{#each options as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
{/snippet}
