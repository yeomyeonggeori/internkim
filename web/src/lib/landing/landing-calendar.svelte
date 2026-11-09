<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CalendarTimeView from '../../routes/calendar/grid/calendar-time-view.svelte';
	import { calendarText } from '../../routes/calendar/text';
	import { sampleEvents } from './landing-samples';

	const text = createPageText(calendarText);
	const today = new Date();
	const hoursPerDay = 24;
	const events = $derived(sampleEvents(currentLocale.value, today));
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const firstEventHour = $derived(Math.min(...events.map((event) => event.start.getHours())));

	let frame = $state<HTMLElement | null>(null);

	$effect(() => {
		const scroller = frame?.querySelector<HTMLElement>('.overflow-y-auto');
		if (!scroller) return;
		scroller.scrollTop = (firstEventHour - 1.25) * (scroller.scrollHeight / hoursPerDay);
	});
</script>

<div bind:this={frame} class="relative flex h-[31rem] min-h-0 flex-col">
	<CalendarTimeView
		visibleDate={today}
		dayCount={7}
		selectedEventID=""
		{events}
		{localeCode}
		draftPreviewTitle=""
		noEventsText={text.noDayEvents}
		showEmptyState={false}
		selectDay={() => {}}
		openEvent={() => {}}
		addEventOnTimeRange={() => {}}
		addEventOnDayRange={() => {}}
	/>
</div>
