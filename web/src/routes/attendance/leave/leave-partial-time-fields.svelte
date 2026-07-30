<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeavePartialPeriod } from './employee-leave-types';
	import type { LeaveRequestDraft } from './leave-request-draft.svelte';

	type Props = {
		draft: LeaveRequestDraft;
		disabled?: boolean;
		field: 'period' | 'startTime';
		text: AttendanceText['leave'];
	};

	let { draft, disabled = false, field, text }: Props = $props();

	const partialPeriods: Array<{ value: EmployeeLeavePartialPeriod; label: string }> = $derived([
		{ value: 'morning', label: text.partialMorning },
		{ value: 'afternoon', label: text.partialAfternoon },
		{ value: 'custom', label: text.partialCustom }
	]);
</script>

<div
	class="grid content-start gap-3"
	data-testid={field === 'period' ? 'leave-partial-time-fields' : 'leave-start-time-field'}
>
	{#if field === 'period'}
		<div class="space-y-1.5">
			<p class="text-sm font-medium">{text.partialPeriodLabel}</p>
			<div class="grid grid-cols-3 rounded-lg border p-0.5">
				{#each partialPeriods as period (period.value)}
					<Button
						type="button"
						variant={draft.partialPeriod === period.value ? 'default' : 'ghost'}
						size="sm"
						class="w-full"
						onclick={() => (draft.partialPeriod = period.value)}
						{disabled}
					>
						{period.label}
					</Button>
				{/each}
			</div>
		</div>
	{/if}

	{#if field === 'startTime'}
		<Field.Field class="gap-1.5">
			<Field.Label for="leave-request-start-time">{text.startTimeLabel}</Field.Label>
			<Input
				id="leave-request-start-time"
				type="time"
				bind:value={draft.startTime}
				required
				{disabled}
			/>
		</Field.Field>
	{/if}
</div>
