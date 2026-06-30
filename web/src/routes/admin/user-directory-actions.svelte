<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import XIcon from '@lucide/svelte/icons/x';
	import type { AdminPageText, UserRecord, UserRole } from './admin-types';

	type UserDirectoryActionsProps = {
		record: UserRecord;
		text: AdminPageText['users'];
		adminCount: number;
		isSaving: boolean;
		isValid: boolean;
		layout: 'desktop' | 'mobile';
		onSave: (record: UserRecord, role?: UserRole) => void;
		onResetPassword: (record: UserRecord) => void;
		onRemove: (email: string) => void;
	};

	let {
		record,
		text,
		adminCount,
		isSaving,
		isValid,
		layout,
		onSave,
		onResetPassword,
		onRemove
	}: UserDirectoryActionsProps = $props();
</script>

<div class={layout === 'desktop' ? 'flex justify-end gap-2' : 'grid gap-2 sm:grid-cols-4'}>
	<Button variant="outline" size="sm" disabled={isSaving || !isValid} onclick={() => onSave(record)}>
		{text.save}
	</Button>
	<Button class="gap-2" variant="outline" size="sm" disabled={isSaving || !isValid} onclick={() => onResetPassword(record)}>
		<RefreshCwIcon class="size-4" />
		{text.resetPassword}
	</Button>
	{#if record.role === 'admin'}
		<Button variant="outline" size="sm" disabled={isSaving || adminCount <= 1 || !isValid} onclick={() => onSave(record, 'member')}>
			{text.makeMember}
		</Button>
	{:else}
		<Button variant="outline" size="sm" disabled={isSaving || !isValid} onclick={() => onSave(record, 'admin')}>
			{text.makeAdmin}
		</Button>
	{/if}
	<Button
		variant="ghost"
		size={layout === 'desktop' ? 'icon-sm' : 'sm'}
		disabled={isSaving || (record.role === 'admin' && adminCount <= 1)}
		onclick={() => onRemove(record.email)}
		aria-label={text.remove}
	>
		<XIcon class="size-4" />
		{#if layout === 'mobile'}
			<span>{text.remove}</span>
		{/if}
	</Button>
</div>
