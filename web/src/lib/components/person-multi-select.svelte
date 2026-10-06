<script lang="ts" module>
	export type SelectablePerson = {
		memberID: string;
		name: string;
		email: string;
		image?: string;
	};
</script>

<script lang="ts">
	import ChevronsUpDownIcon from '@lucide/svelte/icons/chevrons-up-down';
	import XIcon from '@lucide/svelte/icons/x';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonChip from '$lib/components/person-chip.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import * as Empty from '$lib/components/ui/empty';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import * as Popover from '$lib/components/ui/popover';
	import { displayPersonName } from '$lib/person-name.svelte';

	type Props = {
		selectedIDs: string[];
		people: SelectablePerson[];
		label: string;
		placeholder: string;
		onToggle: (memberID: string) => void;
		onRemove?: (memberID: string) => void;
		selectedName?: (memberID: string, index: number) => string;
		canRemove?: (memberID: string) => boolean;
		disabled?: boolean;
		id?: string;
		contentClass?: string;
		side?: 'top' | 'bottom';
	};

	let {
		selectedIDs,
		people,
		label,
		placeholder,
		onToggle,
		onRemove,
		selectedName,
		canRemove,
		disabled = false,
		id,
		contentClass,
		side = 'bottom'
	}: Props = $props();

	let isPickerOpen = $state(false);
	const text = createPageText(appShellText);
	const selected = $derived(new Set(selectedIDs));

	function personOf(memberID: string): SelectablePerson | undefined {
		return people.find((candidate) => candidate.memberID === memberID);
	}

	function nameOf(memberID: string, index: number): string {
		return personOf(memberID)?.name ?? selectedName?.(memberID, index) ?? '';
	}
</script>

<div class="space-y-2">
	<Popover.Root bind:open={isPickerOpen}>
		<Popover.Trigger {disabled}>
			{#snippet child({ props })}
				<Button
					{...props}
					{id}
					{disabled}
					variant="outline"
					role="combobox"
					aria-label={label}
					aria-expanded={isPickerOpen}
					class="w-full justify-between font-normal"
				>
					<span class="truncate text-muted-foreground">{placeholder}</span>
					<ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
				</Button>
			{/snippet}
		</Popover.Trigger>
		<Popover.Content class={contentClass ?? 'w-[var(--bits-popover-anchor-width)] p-0'} align="start" {side}>
			<Command.Root>
				<Command.Input {placeholder} />
				<Command.List>
					<Command.Empty class="p-0"><Empty.Root class="p-3"><Empty.Header><Empty.Title>{text.searchNoResults}</Empty.Title></Empty.Header></Empty.Root></Command.Empty>
					<Command.Group value="people">
						{#each people as person (person.memberID)}
							<Command.Item
								value={person.memberID}
								keywords={[person.name, displayPersonName(person.name), person.email]}
								data-checked={selected.has(person.memberID)}
								onSelect={() => onToggle(person.memberID)}
							>
								<PersonAvatar
									name={displayPersonName(person.name)}
									email={person.email}
									seed={person.memberID}
									image={person.image ?? ''}
									class="size-5"
								/>
								<span class="min-w-0 flex-1">
									<span class="block truncate">{displayPersonName(person.name)}</span>
									<span class="block truncate text-xs text-muted-foreground">{person.email}</span>
								</span>
							</Command.Item>
						{/each}
					</Command.Group>
				</Command.List>
			</Command.Root>
		</Popover.Content>
	</Popover.Root>
	<div class="flex flex-wrap gap-1">
		{#each selectedIDs as memberID, index (memberID)}
			{@const person = personOf(memberID)}
			{@const name = nameOf(memberID, index)}
			{@const email = person?.email ?? ''}
			<PersonChip name={displayPersonName(name)} {email} seed={memberID || name} image={person?.image ?? ''}>
				{#if email}
					<span class="text-[10px] text-muted-foreground">{email}</span>
				{/if}
				{#if onRemove && !disabled && (canRemove?.(memberID) ?? true)}
					<button
						type="button"
						class="-mr-0.5 inline-flex size-4 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
						aria-label={`${displayPersonName(name)} ${label}`}
						onclick={() => onRemove?.(memberID)}
					>
						<XIcon class="size-3" />
					</button>
				{/if}
			</PersonChip>
		{/each}
	</div>
</div>
