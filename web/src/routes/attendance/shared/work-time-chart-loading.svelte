<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { attendanceText } from '../text';

	let { title }: { title: string } = $props();

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	const modes = $derived([
		{ value: 'day', label: text.day },
		{ value: 'week', label: text.week },
		{ value: 'month', label: text.month }
	]);
</script>

<Card.Root class="gap-2" aria-label={title}>
	<Card.Header class="flex flex-col gap-2 space-y-0 pb-0">
		<Card.Title class="flex items-center gap-1.5 text-sm">
			<ClockIcon class="size-3.5 text-muted-foreground" />
			{title}
		</Card.Title>
		<div class="grid w-full grid-cols-3 gap-1 rounded-md border p-0.5">
			{#each modes as mode (mode.value)}
				<Button
					variant={attendance.chartMode === mode.value ? 'default' : 'ghost'}
					size="sm"
					class="h-6 w-full px-0 text-[10px]"
					disabled
				>
					{mode.label}
				</Button>
			{/each}
		</div>
	</Card.Header>
	<Card.Content class="px-3 pb-3 pt-1">
		<Skeleton class="h-24 w-full" />
		<div class="mt-2 flex items-center justify-between border-t pt-2 text-xs">
			<span>{text.recorded} {text.total}</span>
			<Skeleton class="h-4 w-12" />
		</div>
	</Card.Content>
</Card.Root>
