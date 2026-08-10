<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import BriefcaseBusinessIcon from '@lucide/svelte/icons/briefcase-business';
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import ClipboardPenIcon from '@lucide/svelte/icons/clipboard-pen';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import type { CRMRecordKind } from './crm-types';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		onCreate: (kind: CRMRecordKind) => void;
		onImport?: () => void;
	};

	let { text, onCreate, onImport }: Props = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" size="sm" class="shrink-0 gap-2">
				<PlusIcon data-icon="inline-start" />
				{text.quickCreate}
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" sideOffset={8} class="w-56">
		<DropdownMenu.Label>{text.quickCreate}</DropdownMenu.Label>
		<DropdownMenu.Group>
			<DropdownMenu.Item onclick={() => onCreate('relationship')}>
				<Building2Icon data-icon="inline-start" />
				{text.newRelationship}
			</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => onCreate('contact')}>
				<UserPlusIcon data-icon="inline-start" />
				{text.newContact}
			</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => onCreate('progress')}>
				<BriefcaseBusinessIcon data-icon="inline-start" />
				{text.newOpportunity}
			</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => onCreate('activity')}>
				<ClipboardPenIcon data-icon="inline-start" />
				{text.logActivity}
			</DropdownMenu.Item>
			{#if onImport}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={onImport}>{text.importFile}</DropdownMenu.Item>
			{/if}
		</DropdownMenu.Group>
	</DropdownMenu.Content>
</DropdownMenu.Root>
