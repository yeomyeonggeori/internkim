<script lang="ts">
	import { page } from '$app/state';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		makeCalendarSubscriptionURL,
		readCalendarSubscription,
		revokeCalendarSubscriptionURL
	} from './calendar-layout-api';
	import { CalendarLayoutState } from './calendar-layout-state.svelte';
	import CalendarLayoutSyncSheet from './calendar-layout-sync-sheet.svelte';
	import type { CalendarSyncResponse } from './calendar-layout-types';
	import { calendarText } from './text';
	import { isEmbeddedCalendar } from '$lib/app-shell';
	import { setCalendarSettings } from './calendar-settings-context';

	let { children } = $props();

	const text = createPageText(calendarText);
	const isEmbed = $derived(isEmbeddedCalendar(page.url.pathname));
	const layoutState = new CalendarLayoutState();
	setCalendarSettings(openSyncSheet);

	async function loadSubscription() {
		try {
			layoutState.syncInformation = await readCalendarSubscription();
		} catch {
			layoutState.syncInformation = null;
		}
	}

	async function saveSubscription(change: () => Promise<CalendarSyncResponse>) {
		layoutState.isSavingSubscription = true;
		layoutState.syncError = '';
		try {
			layoutState.syncInformation = await change();
		} catch {
			layoutState.syncError = text.saveError;
		} finally {
			layoutState.isSavingSubscription = false;
		}
	}

	function openSyncSheet() {
		layoutState.openSyncSheet();
		loadSubscription();
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
		isSavingSubscription={layoutState.isSavingSubscription}
		syncError={layoutState.syncError}
		makeSubscriptionURL={() => saveSubscription(makeCalendarSubscriptionURL)}
		revokeSubscriptionURL={() => saveSubscription(revokeCalendarSubscriptionURL)}
	/>
{/if}
