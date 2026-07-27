<script lang="ts">
	import { Button, type ButtonSize, type ButtonVariant } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import FilterIcon from '@lucide/svelte/icons/filter';
	import { Popover } from 'bits-ui';
	import { allValue } from './organization-directory-controller.svelte';
	import { unassignedGroupID, type OrganizationDirectoryOptions } from './organization-directory-model';
	import type { organizationDirectoryText } from './text';

	type OrganizationFilterText = Pick<(typeof organizationDirectoryText)['ko'], 'filter' | 'organization' | 'allOrganizations' | 'unassignedTeam'>;

	type OrganizationFilterPopoverProps = {
		isOpen?: boolean;
		selectedGroupID?: string;
		options: OrganizationDirectoryOptions;
		text: OrganizationFilterText;
		buttonSize?: ButtonSize;
		buttonVariant?: ButtonVariant;
		onSelectGroup: (groupID: string) => void;
	};

	let {
		isOpen = $bindable(false),
		selectedGroupID = '',
		options,
		text,
		buttonSize = 'default',
		buttonVariant = 'outline',
		onSelectGroup
	}: OrganizationFilterPopoverProps = $props();

	const selectedOrganizationLabel = $derived(
		selectedGroupID === unassignedGroupID
			? text.unassignedTeam
			: selectedGroupID
				? options.groups.find((group) => group.id === selectedGroupID)?.name ?? text.allOrganizations
				: text.allOrganizations
	);

	function selectGroup(groupID: string): void {
		onSelectGroup(groupID);
		isOpen = false;
	}
</script>

<Popover.Root bind:open={isOpen}>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" size={buttonSize} variant={buttonVariant} aria-pressed={isOpen}>
				<FilterIcon class="size-4" />
				{text.filter}
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			align="end"
			sideOffset={8}
			data-testid="organization-filter-popover"
			class="z-50 w-[min(20rem,calc(100vw-2rem))] rounded-lg border bg-popover p-3 text-popover-foreground shadow-lg outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95"
		>
			<div class="grid gap-1.5">
				<Label>{text.organization}</Label>
				<Select.Root type="single" value={selectedGroupID || allValue} onValueChange={selectGroup}>
					<Select.Trigger class="w-full" aria-label={text.organization}>
						{selectedOrganizationLabel}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value={allValue} label={text.allOrganizations}>{text.allOrganizations}</Select.Item>
						{#each options.groups as group}
							<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
						{/each}
						{#if options.hasUnassigned}
							<Select.Item value={unassignedGroupID} label={text.unassignedTeam}>{text.unassignedTeam}</Select.Item>
						{/if}
					</Select.Content>
				</Select.Root>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
