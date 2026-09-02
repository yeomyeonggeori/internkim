<script lang="ts">
	import { page } from '$app/state';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import {
		fetchCalendarSyncInformation,
		issueCalendarSubscriptionURL,
		rotateCalendarSubscriptionURL
	} from './calendar-layout-api';
	import { installCalendarLayoutMessageSync } from './calendar-layout-message-sync';
	import { CalendarLayoutState } from './calendar-layout-state.svelte';
	import CalendarLayoutSyncSheet from './calendar-layout-sync-sheet.svelte';
	import { bumpCalendarRefresh } from './refresh-signal.svelte';
	import { calendarText } from './text';
	import { isEmbeddedCalendar } from '$lib/app-shell';

	let { children } = $props();

	const text = createPageText(calendarText);
	const isEmbed = $derived(isEmbeddedCalendar(page.url.pathname));
	const layoutState = new CalendarLayoutState();

	onMount(() => {
		if (isEmbed) return;
		loadSyncInformation();
		return installCalendarLayoutMessageSync({
			selectedDateKey: () => layoutState.selectedDateKey,
			applyVisibleDate: (date) => layoutState.applyVisibleDate(date),
			openSettings: openSyncSheet,
			refreshCalendar: bumpCalendarRefresh
		});
	});

	async function loadSyncInformation() {
		try {
			layoutState.syncInformation = await fetchCalendarSyncInformation();
		} catch {
			layoutState.syncInformation = null;
		}
	}

	async function loadSubscriptionURL() {
		try {
			layoutState.syncInformation = await issueCalendarSubscriptionURL();
		} catch {
			layoutState.syncInformation = null;
		}
	}

	async function rotateSubscriptionURL() {
		layoutState.isRotatingSync = true;
		layoutState.syncError = '';
		try {
			layoutState.syncInformation = await rotateCalendarSubscriptionURL();
		} catch {
			layoutState.syncError = text.saveError;
		} finally {
			layoutState.isRotatingSync = false;
		}
	}

	function openSyncSheet() {
		layoutState.openSyncSheet();
		loadSubscriptionURL();
	}
</script>

{#if isEmbed}
	{@render children()}
{:else}
	<div class="flex min-w-0 flex-1 flex-col">
		{@render children()}
	</div>

	<CalendarLayoutSyncSheet
		bind:isOpen={layoutState.isSyncSheetOpen}
		{text}
		syncInformation={layoutState.syncInformation}
		isRotatingSync={layoutState.isRotatingSync}
		syncError={layoutState.syncError}
		{rotateSubscriptionURL}
	/>
{/if}
