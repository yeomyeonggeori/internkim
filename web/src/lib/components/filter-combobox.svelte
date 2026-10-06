<script lang="ts" module>
	export type FilterComboboxOption = {
		value: string;
		label: string;
		keywords?: string[];
	};
</script>

<script lang="ts" generics="Option extends FilterComboboxOption">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Command from '$lib/components/ui/command/index.js';
	import * as Empty from '$lib/components/ui/empty';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils.js';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import { tick } from 'svelte';

	let {
		value = $bindable(''),
		options,
		label,
		searchPlaceholder,
		clearValue = '',
		canClearSelection = true,
		searchable = true,
		disabled = false,
		id,
		class: className,
		contentClass,
		icon,
		onSelect,
		optionContent,
		selectedContent,
		onSearchChange,
		remoteSearch = false,
		isSearching = false,
		searchError = '',
		searchStatusLabel
	}: {
		value?: string;
		options: Option[];
		label: string;
		searchPlaceholder?: string;
		clearValue?: string;
		canClearSelection?: boolean;
		searchable?: boolean;
		disabled?: boolean;
		id?: string;
		class?: string;
		contentClass?: string;
		icon?: Snippet;
		onSelect?: (value: string) => void;
		optionContent?: Snippet<[Option]>;
		selectedContent?: Snippet<[Option]>;
		onSearchChange?: (query: string) => void;
		remoteSearch?: boolean;
		isSearching?: boolean;
		searchError?: string;
		searchStatusLabel?: string;
	} = $props();

	const text = createPageText(appShellText);

	let open = $state(false);
	let triggerRef = $state<HTMLButtonElement>(null!);

	const selectedOption = $derived(options.find((option) => option.value === value && option.value !== clearValue));

	function selectOption(optionValue: string) {
		const nextValue = canClearSelection && value === optionValue ? clearValue : optionValue;
		value = nextValue;
		onSelect?.(nextValue);
		closeAndFocusTrigger();
	}

	function closeAndFocusTrigger() {
		open = false;
		tick().then(() => {
			triggerRef.focus();
		});
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger bind:ref={triggerRef}>
		{#snippet child({ props })}
			<Button
				{...props}
				{disabled}
				{id}
				variant="outline"
				class={cn('w-[200px] justify-between', className)}
				role="combobox"
				aria-label={label}
				aria-expanded={open}
			>
				<span class="flex min-w-0 items-center gap-2">
					{#if !(selectedOption && selectedContent)}
						{@render icon?.()}
					{/if}
					{#if !selectedOption}
						{label}
					{:else if selectedContent}
						{@render selectedContent(selectedOption)}
					{:else}
						<span class="truncate">{label}: {selectedOption.label}</span>
					{/if}
				</span>
				<ChevronsUpDownIcon class="opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class={cn('w-[200px] p-0', contentClass)}>
		<Command.Root shouldFilter={!remoteSearch}>
			{#if searchable}
				<Command.Input placeholder={searchPlaceholder ?? text.search} oninput={(event) => onSearchChange?.(event.currentTarget.value)} />
			{/if}
			<Command.List>
				{#if isSearching}
					<div role="status" aria-label={searchStatusLabel ?? label} aria-busy="true" class="flex justify-center p-3"><Spinner /></div>
				{:else if searchError}
					<p role="alert" class="p-3 text-sm text-destructive">{searchError}</p>
				{:else}
					<Command.Empty class="p-0"><Empty.Root class="p-3"><Empty.Header><Empty.Title>{text.searchNoResults}</Empty.Title></Empty.Header></Empty.Root></Command.Empty>
				{/if}
				<Command.Group value="options">
					{#each options as option (option.value)}
						<Command.Item
							value={option.value}
							keywords={[option.label, ...(option.keywords ?? [])]}
							data-checked={value === option.value}
							onSelect={() => selectOption(option.value)}
						>
							{#if optionContent}
								{@render optionContent(option)}
							{:else}
								{option.label}
							{/if}
						</Command.Item>
					{/each}
				</Command.Group>
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
