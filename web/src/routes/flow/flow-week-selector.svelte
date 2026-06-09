<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import { Popover } from 'bits-ui';
	import { flowWeekCodeForDateISO, formatFlowWeekDateRange, type FlowWeekDateRange } from './flow-week-label';

	type Props = {
		week: FlowWeekDateRange | null | undefined;
		disabled: boolean;
		selectDateLabel: string;
		currentWeekLabel: string;
		onSelectWeek: (weekCode: string) => void;
		onSelectCurrentWeek: () => void;
	};

	let { week, disabled, selectDateLabel, currentWeekLabel, onSelectWeek, onSelectCurrentWeek }: Props = $props();
	let isOpen = $state(false);

	const dateInputValue = () => week?.startISO || '';

	function selectDate(dateISO: string) {
		const weekCode = flowWeekCodeForDateISO(dateISO);
		if (!weekCode) return;
		onSelectWeek(weekCode);
		isOpen = false;
	}

	function handleDateInputChange(event: Event) {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		selectDate(event.currentTarget.value);
	}

	function moveToCurrentWeek() {
		onSelectCurrentWeek();
		isOpen = false;
	}
</script>

<Popover.Root bind:open={isOpen}>
	<Popover.Trigger
		{disabled}
		aria-label={selectDateLabel}
		class="inline-flex h-8 w-[7.5rem] shrink-0 items-center justify-center rounded-[min(var(--radius-md),10px)] border border-border bg-card px-2.5 text-sm font-medium tabular-nums shadow-xs outline-none transition-all hover:bg-muted hover:text-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50"
	>
		{formatFlowWeekDateRange(week)}
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			sideOffset={8}
			class="z-50 w-64 rounded-lg border bg-popover p-3 text-popover-foreground shadow-md outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95"
		>
			<div class="space-y-3">
				<div class="flex items-center gap-2 text-sm font-medium">
					<CalendarDaysIcon class="size-4 text-muted-foreground" />
					<span>{selectDateLabel}</span>
				</div>
				<Input type="date" value={dateInputValue()} onchange={handleDateInputChange} />
				<Button variant="secondary" size="sm" class="w-full" onclick={moveToCurrentWeek}>
					{currentWeekLabel}
				</Button>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
