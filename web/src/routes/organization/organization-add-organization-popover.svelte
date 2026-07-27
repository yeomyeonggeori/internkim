<script lang="ts">
	import { Button, type ButtonSize, type ButtonVariant } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import type { OrgGroup } from '$lib/organization/types';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { Popover } from 'bits-ui';
	import type { AdminPageText } from '../admin/admin-types';

	type OrganizationAddOrganizationText = Pick<AdminPageText['organization'], 'addOrganization' | 'newGroup' | 'groupPlaceholder' | 'addGroup' | 'cancel' | 'parentOrganization' | 'rootOrganization'>;

	type OrganizationAddOrganizationPopoverProps = {
		isOpen?: boolean;
		newGroupName?: string;
		newGroupParentID?: string;
		groups: OrgGroup[];
		isSaving: boolean;
		text: OrganizationAddOrganizationText;
		inputID: string;
		buttonSize?: ButtonSize;
		buttonVariant?: ButtonVariant;
		onAdd: () => void | Promise<void>;
		onCancel: () => void;
		onOpenChange: (isOpen: boolean) => void;
	};

	let {
		isOpen = $bindable(false),
		newGroupName = $bindable(''),
		newGroupParentID = $bindable(''),
		groups,
		isSaving,
		text,
		inputID,
		buttonSize = 'default',
		buttonVariant = 'default',
		onAdd,
		onCancel,
		onOpenChange
	}: OrganizationAddOrganizationPopoverProps = $props();

	const triggerVariant = $derived(isOpen ? 'outline' : buttonVariant);
	const rootValue = '__root__';
	const selectedParentName = $derived(groups.find((group) => group.id === newGroupParentID)?.name ?? text.rootOrganization);

	function submitNewOrganization(event: SubmitEvent): void {
		event.preventDefault();
		void onAdd();
	}

	function selectParent(value: string | undefined): void {
		newGroupParentID = value === rootValue || value === undefined ? '' : value;
	}
</script>

<Popover.Root bind:open={isOpen} onOpenChange={onOpenChange}>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" size={buttonSize} variant={triggerVariant} aria-pressed={isOpen}>
				<PlusIcon class="size-4" />
				{text.addOrganization}
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			align="end"
			sideOffset={8}
			data-testid="organization-add-organization-popover"
			class="z-50 w-[min(24rem,calc(100vw-2rem))] rounded-lg border bg-popover p-3 text-popover-foreground shadow-lg outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95"
		>
			<form class="grid gap-4" onsubmit={submitNewOrganization}>
				<div class="grid gap-1.5">
					<Label class="text-xs" for={inputID}>{text.newGroup}</Label>
					<Input id={inputID} class="h-9 min-w-0 text-sm" bind:value={newGroupName} placeholder={text.groupPlaceholder} autocomplete="off" disabled={isSaving} />
				</div>
				<div class="grid gap-1.5">
					<Label class="text-xs">{text.parentOrganization}</Label>
					<Select.Root type="single" value={newGroupParentID || rootValue} onValueChange={selectParent} disabled={isSaving}>
						<Select.Trigger class="w-full" aria-label={text.parentOrganization}>{selectedParentName}</Select.Trigger>
						<Select.Content>
							<Select.Item value={rootValue} label={text.rootOrganization}>{text.rootOrganization}</Select.Item>
							{#each groups as group}
								<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="flex justify-end gap-2">
						<Button type="button" size="sm" variant="outline" disabled={isSaving} onclick={onCancel}>{text.cancel}</Button>
						<Button type="submit" size="sm" disabled={isSaving || !newGroupName.trim()} class="gap-2">
							<PlusIcon class="size-4" />
							{text.addGroup}
						</Button>
				</div>
			</form>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
