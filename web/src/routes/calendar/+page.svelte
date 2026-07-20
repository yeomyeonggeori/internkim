<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import CalendarPage from './embed/calendar-page.svelte';
	import { calendarRefresh } from './refresh-signal.svelte';

	const calendarPageKey = $derived(`${calendarRefresh.ticks}-${currentLocale.value}`);
</script>

{#key calendarPageKey}
	<CalendarPage />
{/key}
