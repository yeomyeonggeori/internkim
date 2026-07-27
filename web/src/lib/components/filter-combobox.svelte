<script lang="ts" module>
	export type FilterComboboxOption = {
		value: string;
		label: string;
	};
</script>

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';

	let {
		value = $bindable(''),
		options,
		label,
		clearValue = '',
		class: className
	}: {
		value?: string;
		options: FilterComboboxOption[];
		label: string;
		clearValue?: string;
		class?: string;
	} = $props();

	const text = createPageText(appShellText);
	let open = $state(false);

	const selectedLabel = $derived(options.find((option) => option.value === value && option.value !== clearValue)?.label ?? '');

	function selectOption(optionValue: string) {
		value = value === optionValue ? clearValue : optionValue;
		open = false;
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="outline" size="sm" class={className}>
				<span class="truncate">{selectedLabel || label}</span>
				<ChevronsUpDownIcon class="ml-auto opacity-50" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content class="w-56 p-0" align="start">
		<Command.Root>
			<Command.Input placeholder={label} />
			<Command.List>
				<Command.Empty>{text.searchNoResults}</Command.Empty>
				{#each options as option (option.value)}
					<Command.Item value={option.value} keywords={[option.label]} onSelect={() => selectOption(option.value)}>
						{option.label}
						{#if value === option.value}
							<CheckIcon class="ml-auto" />
						{/if}
					</Command.Item>
				{/each}
			</Command.List>
		</Command.Root>
	</Popover.Content>
</Popover.Root>
