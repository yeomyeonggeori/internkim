<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import CalendarIcon from '@lucide/svelte/icons/calendar-days';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { CalendarViewValue } from './calendar-navigation-message';
	import CalendarMiniMonth from './calendar-mini-month.svelte';
	import type { CalendarSource } from './calendar-layout-types';

	type CalendarSidebarText = {
		title: string;
		moreOptions: string;
		myCalendars: string;
		addCalendar: string;
		calendarVisibility: string;
		subscriptionSettings: string;
		refresh: string;
	};

	let {
		text,
		calendarSources,
		miniMonth,
		miniMonthCalendarView,
		miniMonthEventDates,
		selectedMiniDateKey,
		setMiniMonth,
		selectMiniMonthDate,
		openSyncSheet,
		refreshCalendar,
		toggleCalendarSource
	}: {
		text: CalendarSidebarText;
		calendarSources: CalendarSource[];
		miniMonth: Date;
		miniMonthCalendarView: CalendarViewValue;
		miniMonthEventDates: Set<string>;
		selectedMiniDateKey: string;
		setMiniMonth: (month: Date) => void;
		selectMiniMonthDate: (date: Date) => void;
		openSyncSheet: () => void;
		refreshCalendar: () => void;
		toggleCalendarSource: (sourceID: string) => void;
	} = $props();
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<CalendarIcon class="size-5 text-foreground" />
		<div class="min-w-0 flex-1">
			<p class="truncate text-[15px] font-semibold tracking-tight">{text.title}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.moreOptions} class="size-8">•••</Button>
	</div>

	<div class="min-h-0 flex-1 overflow-auto">
		<section class="space-y-3 px-5 py-5">
			<header class="flex items-center justify-between px-1">
				<p class="text-[12px] font-bold uppercase tracking-[0.14em] text-muted-foreground">{text.myCalendars}</p>
				<button type="button" aria-label={text.addCalendar} class="flex size-5 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-accent-foreground">
					<PlusIcon class="size-3.5" />
				</button>
			</header>
			<div class="space-y-2">
				{#each calendarSources as source (source.id)}
					<div class="flex h-9 w-full items-center gap-3 rounded-md text-left text-[15px]">
						<button
							type="button"
							aria-pressed={source.isChecked}
							aria-label={`${source.label} ${text.calendarVisibility}`}
							class="flex size-5 shrink-0 items-center justify-center rounded-md border-2 {source.isChecked
								? source.colorClass
								: 'border-[#a9c89e] bg-transparent text-transparent'}"
							onclick={() => toggleCalendarSource(source.id)}
						>
							<span class="text-[15px] font-bold leading-none {source.isChecked ? '' : 'opacity-0'}">✓</span>
						</button>
						<span class="min-w-0 flex-1 truncate text-foreground">{source.label}</span>
						{#if source.count}
							<span class="text-sm tabular-nums text-muted-foreground">{source.count}</span>
						{/if}
					</div>
				{/each}
			</div>
		</section>
	</div>

	<CalendarMiniMonth
		month={miniMonth}
		calendarView={miniMonthCalendarView}
		eventDates={miniMonthEventDates}
		selectedDateKey={selectedMiniDateKey}
		onMonthChange={setMiniMonth}
		onSelectDate={selectMiniMonthDate}
	/>

	<div class="flex shrink-0 items-center gap-1 border-t px-2 py-2 text-xs">
		<button
			type="button"
			class="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent"
			onclick={openSyncSheet}
		>
			<span class="size-1.5 rounded-full bg-muted-foreground/50" aria-hidden="true"></span>
			<span class="min-w-0 flex-1 truncate text-muted-foreground">{text.subscriptionSettings}</span>
		</button>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} class="size-7" onclick={refreshCalendar}>
			<RefreshCwIcon class="size-3.5" />
		</Button>
	</div>
</aside>
