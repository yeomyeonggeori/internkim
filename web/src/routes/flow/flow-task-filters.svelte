<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Option = {
		value: string;
		label: string;
	};

	type Props = {
		searchText: string;
		statusFilter: string;
		ownerFilter: string;
		businessFilter: string;
		typeFilter: string;
		statusOptions: Option[];
		ownerOptions: Option[];
		businessOptions: Option[];
		typeOptions: Option[];
		hasBusinessFilter: boolean;
		hasMembers: boolean;
		text: FlowPageText['filters'];
		resetFilters: () => void;
		createTask: () => void;
	};

	let {
		searchText = $bindable(''),
		statusFilter = $bindable('all'),
		ownerFilter = $bindable('all'),
		businessFilter = $bindable('all'),
		typeFilter = $bindable('all'),
		statusOptions,
		ownerOptions,
		businessOptions,
		typeOptions,
		hasBusinessFilter,
		hasMembers,
		text,
		resetFilters,
		createTask
	}: Props = $props();
</script>

<div class="flex flex-wrap items-end gap-2 rounded-lg border bg-card p-3">
	<div class="relative grow basis-56">
		<SearchIcon class="absolute left-2 top-2.5 size-4 text-muted-foreground" />
		<Input class="pl-8" placeholder={text.searchPlaceholder} bind:value={searchText} />
	</div>
	<div class="grow basis-36">
		{@render SelectControl(text.status, statusOptions, statusFilter, (value: string) => (statusFilter = value))}
	</div>
	<div class="grow basis-36">
		{@render SelectControl(text.owner, ownerOptions, ownerFilter, (value: string) => (ownerFilter = value))}
	</div>
	{#if hasBusinessFilter}
		<div class="grow basis-36">
			{@render SelectControl(text.business, businessOptions, businessFilter, (value: string) => (businessFilter = value))}
		</div>
	{/if}
	<div class="grow basis-36">
		{@render SelectControl(text.type, typeOptions, typeFilter, (value: string) => (typeFilter = value))}
	</div>
	<Button variant="ghost" size="sm" onclick={resetFilters}>{text.reset}</Button>
	<Button size="sm" onclick={() => createTask()} disabled={!hasMembers}>
		<PlusIcon />
		{text.addTask}
	</Button>
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
