<script lang="ts">
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import DatePicker from '$lib/components/date-picker.svelte';
	import DateTimePicker from '$lib/components/date-time-picker.svelte';
	import { activityDatesTurned } from './crm-activity-dates';
	import type { CRMText } from './text';

	type Props = {
		idPrefix: string;
		isWholeDay: boolean;
		startsAt: string;
		endsAt: string;
		text: CRMText;
	};

	let { idPrefix, isWholeDay = $bindable(true), startsAt = $bindable(''), endsAt = $bindable(''), text }: Props = $props();

	function turnWholeDay(checked: boolean): void {
		({ isWholeDay, startsAt, endsAt } = activityDatesTurned({ isWholeDay, startsAt, endsAt }, checked));
	}
</script>

<Field.Field orientation="horizontal"><Checkbox id="{idPrefix}-all-day" checked={isWholeDay} onCheckedChange={turnWholeDay} /><Field.Content><Field.Label for="{idPrefix}-all-day">{text.allDay}</Field.Label></Field.Content></Field.Field>
<div class="grid gap-4 sm:grid-cols-2">
	{#if isWholeDay}
		<Field.Field><Field.Label for="{idPrefix}-start">{text.columnStartDate}</Field.Label><DatePicker id="{idPrefix}-start" bind:value={startsAt} /></Field.Field>
		<Field.Field><Field.Label for="{idPrefix}-end">{text.columnEndDate}</Field.Label><DatePicker id="{idPrefix}-end" bind:value={endsAt} /></Field.Field>
	{:else}
		<Field.Field><Field.Label for="{idPrefix}-start">{text.startTime}</Field.Label><DateTimePicker id="{idPrefix}-start" bind:value={startsAt} /></Field.Field>
		<Field.Field><Field.Label for="{idPrefix}-end">{text.endTime}</Field.Label><DateTimePicker id="{idPrefix}-end" bind:value={endsAt} /></Field.Field>
	{/if}
</div>
