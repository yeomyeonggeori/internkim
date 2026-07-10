<script lang="ts">
	import { Button, type ButtonSize, type ButtonVariant } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { Popover } from 'bits-ui';
	import type { AdminPageText } from '../admin/admin-types';

	type OrgchartAddOrganizationText = Pick<AdminPageText['orgchart'], 'addOrganization' | 'newGroup' | 'groupPlaceholder' | 'addGroup' | 'cancel'>;

	type OrgchartAddOrganizationPopoverProps = {
		isOpen?: boolean;
		newGroupName?: string;
		isSaving: boolean;
		text: OrgchartAddOrganizationText;
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
		isSaving,
		text,
		inputID,
		buttonSize = 'default',
		buttonVariant = 'default',
		onAdd,
		onCancel,
		onOpenChange
	}: OrgchartAddOrganizationPopoverProps = $props();

	const triggerVariant = $derived(isOpen ? 'outline' : buttonVariant);

	function submitNewOrganization(event: SubmitEvent): void {
		event.preventDefault();
		void onAdd();
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
			data-testid="orgchart-add-organization-popover"
			class="z-50 w-[min(24rem,calc(100vw-2rem))] rounded-lg border bg-popover p-3 text-popover-foreground shadow-lg outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95"
		>
			<form class="grid gap-2" onsubmit={submitNewOrganization}>
				<Label class="text-xs" for={inputID}>{text.newGroup}</Label>
				<div class="flex flex-col gap-2 sm:flex-row">
					<Input id={inputID} class="h-8 min-w-0 flex-1 text-sm" bind:value={newGroupName} placeholder={text.groupPlaceholder} autocomplete="off" disabled={isSaving} />
					<div class="flex shrink-0 gap-2">
						<Button type="submit" size="sm" disabled={isSaving || !newGroupName.trim()} class="h-8 flex-1 gap-2 sm:flex-none">
							<PlusIcon class="size-4" />
							{text.addGroup}
						</Button>
						<Button type="button" size="sm" variant="outline" disabled={isSaving} class="h-8 flex-1 sm:flex-none" onclick={onCancel}>
							{text.cancel}
						</Button>
					</div>
				</div>
			</form>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
