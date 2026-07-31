<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { buttonVariants } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { attendanceText } from '../text';
	import { fetchLegacyAbsenceMigrationPreview } from './leave-management-api';
	import { getLeaveManagementState } from './leave-management-state.svelte';
	import type { LeaveManagementLegacyMigrationPreview } from './leave-management-types';

	const text = createPageText(attendanceText);
	const management = getLeaveManagementState();

	let preview = $state<LeaveManagementLegacyMigrationPreview | null>(null);
	let isOpen = $state(false);
	let isApplying = $state(false);

	const isVisible = $derived(
		(preview?.candidateLeaveCount ?? 0) > 0 || (preview?.conflictCount ?? 0) > 0
	);
	const isBlocked = $derived((preview?.conflictCount ?? 0) > 0);

	function countText(template: string, count: number): string {
		return template.replace('{count}', String(count));
	}

	async function loadPreview(showError: boolean): Promise<void> {
		try {
			preview = await fetchLegacyAbsenceMigrationPreview();
		} catch {
			if (showError) toast.error(text.management.legacyMigrationLoadFailed);
		}
	}

	async function applyMigration(): Promise<void> {
		if (!preview || isBlocked || isApplying || management.isMutating) return;
		const migratedCount = preview.candidateLeaveCount;
		isApplying = true;
		try {
			await management.migrateLegacyAbsences(preview.fingerprint);
			preview = null;
			await loadPreview(false);
			isOpen = false;
			toast.success(
				countText(text.management.legacyMigrationCompletedTemplate, migratedCount)
			);
		} catch {
			toast.error(text.management.legacyMigrationApplyFailed);
			await loadPreview(false);
		} finally {
			isApplying = false;
		}
	}

	onMount(() => {
		void loadPreview(true);
	});
</script>

{#if isVisible && preview}
	<AlertDialog.Root bind:open={isOpen}>
		<AlertDialog.Trigger
			class={buttonVariants({ variant: 'outline' })}
			data-testid="legacy-leave-migration-action"
			disabled={management.isMutating}
		>
			{text.management.legacyMigrationAction}
		</AlertDialog.Trigger>
		<AlertDialog.Content data-testid="legacy-leave-migration-dialog">
			<AlertDialog.Header>
				<AlertDialog.Title>
					{isBlocked
						? text.management.legacyMigrationBlockedTitle
						: text.management.legacyMigrationTitle}
				</AlertDialog.Title>
				<AlertDialog.Description>
					{isBlocked
						? countText(
								text.management.legacyMigrationBlockedTemplate,
								preview.conflictCount
							)
						: countText(
								text.management.legacyMigrationConfirmTemplate,
								preview.candidateLeaveCount
							)}
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel disabled={isApplying || management.isMutating}>
					{isBlocked ? text.approval.close : text.cancel}
				</AlertDialog.Cancel>
				{#if !isBlocked}
					<AlertDialog.Action
						onclick={() => void applyMigration()}
						disabled={isApplying || management.isMutating}
						loading={isApplying}
					>
						{text.management.legacyMigrationConfirmAction}
					</AlertDialog.Action>
				{/if}
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
