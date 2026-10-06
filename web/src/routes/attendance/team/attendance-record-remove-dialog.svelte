<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import ChangeReasonSelect from '../shared/change-reason-select.svelte';
	import type { AttendanceText } from '../text';
	import type { AttendanceRecordRemovalState } from './attendance-record-removal.svelte';
	import { attendanceWriteIntent, attendanceWriteSubmitLabel } from './attendance-write-notice';

	type Props = {
		text: AttendanceText;
		removal: AttendanceRecordRemovalState;
	};

	let { text, removal }: Props = $props();

	const outcome = $derived(removal.outcome);
	const noticeMessage = $derived(attendanceWriteIntent(outcome, text.records));
	const submitLabel = $derived(
		attendanceWriteSubmitLabel(outcome, text.records, text.records.removeSubmit)
	);
	const removedRecord = $derived(describeRemovedRecord());

	function describeRemovedRecord(): string {
		const event = removal.event;
		if (!event) return '';
		const kindLabel =
			event.kind === 'clock_in' ? text.records.clockInLabel : text.records.clockOutLabel;
		return `${event.localDate} ${event.localTime} · ${kindLabel}`;
	}
</script>

<Dialog.Root open={removal.isOpen} onOpenChange={(open) => (open ? undefined : removal.close())}>
	<Dialog.Content class="sm:max-w-md" closeLabel={text.close}>
		<Dialog.Header>
			<Dialog.Title>{text.records.removeTitle}</Dialog.Title>
			<Dialog.Description>{text.records.removeDescription}</Dialog.Description>
		</Dialog.Header>

		<div class="grid gap-3 py-2">
			<div class="grid gap-1 rounded-lg border border-border/70 bg-muted/40 px-3 py-2">
				<span class="text-[11px] font-medium text-muted-foreground">
					{text.records.removedRecord}
				</span>
				<span class="text-sm font-medium" data-testid="attendance-record-remove-target">
					{removedRecord}
				</span>
			</div>

			<ChangeReasonSelect bind:value={removal.reason} disabled={removal.isSaving} label={text.records.reason} />

			<p class="text-xs text-muted-foreground" data-testid="attendance-record-remove-notice">
				{noticeMessage}
			</p>
			{#if removal.errorMessage}
				<p class="text-xs text-destructive">{removal.errorMessage}</p>
			{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" disabled={removal.isSaving} onclick={() => removal.close()}>
				{text.cancel}
			</Button>
			<Button
				variant="destructive"
				disabled={!removal.canSubmit}
				onclick={() => removal.submit()}
				data-testid="attendance-record-remove-submit"
			>
				{submitLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
