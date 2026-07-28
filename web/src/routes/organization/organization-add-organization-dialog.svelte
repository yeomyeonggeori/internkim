<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import type { OrgGroup } from '$lib/organization/types';
	import type { AdminPageText } from '../admin/admin-types';

	type OrganizationAddOrganizationText = Pick<
		AdminPageText['organization'],
		'addOrganization' | 'newGroup' | 'groupPlaceholder' | 'addGroup' | 'cancel' | 'parentOrganization' | 'rootOrganization'
	>;

	let {
		isOpen = $bindable(false),
		newGroupName = $bindable(''),
		newGroupParentID = $bindable(''),
		groups,
		isSaving,
		text,
		inputID,
		onAdd,
		onCancel,
		onOpenChange
	}: {
		isOpen?: boolean;
		newGroupName?: string;
		newGroupParentID?: string;
		groups: OrgGroup[];
		isSaving: boolean;
		text: OrganizationAddOrganizationText;
		inputID: string;
		onAdd: () => void | Promise<void>;
		onCancel: () => void;
		onOpenChange: (isOpen: boolean) => void;
	} = $props();

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

<Dialog.Root bind:open={isOpen} {onOpenChange}>
	<Dialog.Content class="sm:max-w-md" data-testid="organization-add-organization-popover">
		<Dialog.Header>
			<Dialog.Title>{text.addOrganization}</Dialog.Title>
		</Dialog.Header>
		<form class="grid gap-4" onsubmit={submitNewOrganization}>
			<div class="grid gap-1.5">
				<Label for={inputID}>{text.newGroup}</Label>
				<Input id={inputID} bind:value={newGroupName} placeholder={text.groupPlaceholder} autocomplete="off" disabled={isSaving} />
			</div>
			<div class="grid gap-1.5">
				<Label>{text.parentOrganization}</Label>
				<Select.Root type="single" value={newGroupParentID || rootValue} onValueChange={selectParent} disabled={isSaving}>
					<Select.Trigger class="w-full" aria-label={text.parentOrganization}>{selectedParentName}</Select.Trigger>
					<Select.Content>
						<Select.Item value={rootValue} label={text.rootOrganization}>{text.rootOrganization}</Select.Item>
						{#each groups as group (group.id)}
							<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" disabled={isSaving} onclick={onCancel}>{text.cancel}</Button>
				<Button type="submit" disabled={isSaving || !newGroupName.trim()}>{text.addGroup}</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
