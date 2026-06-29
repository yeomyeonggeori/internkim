<script lang="ts">
	import { page } from '$app/state';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { onMount } from 'svelte';
	import { calendarEventDateKeysForMonth } from './calendar-layout-date';
	import {
		fetchCalendarAccountStatus,
		fetchCalendarEventsForMonth,
		fetchCalendarSyncInformation,
		rotateCalendarSubscriptionURL,
		uploadGoogleOAuthClient
	} from './calendar-layout-api';
	import {
		initialSelectedDateKey,
		installCalendarLayoutMessageSync,
		loadCalendarLayoutStoredState,
		saveAndBroadcastCalendarWorkVisibility
	} from './calendar-layout-message-sync';
	import CalendarLayoutSidebar from './calendar-layout-sidebar.svelte';
	import { CalendarLayoutState } from './calendar-layout-state.svelte';
	import CalendarLayoutSyncSheet from './calendar-layout-sync-sheet.svelte';
	import { broadcastCalendarNavigation, bumpCalendarRefresh, calendarVisibility } from './refresh-signal.svelte';
	import { calendarText } from './text';

	let { children } = $props();

	const text = createPageText(calendarText);
	const isEmbed = $derived(page.url.pathname.startsWith('/calendar/embed'));
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');
	const layoutState = new CalendarLayoutState();

	const todayMonthLabel = $derived(
		layoutState.today.toLocaleDateString(localeCode, {
			year: 'numeric',
			month: 'long'
		})
	);

	function selectMiniMonthDate(date: Date) {
		layoutState.selectMiniMonthDate(date);
		broadcastCalendarNavigation(new Date(date.getFullYear(), date.getMonth(), date.getDate(), 12, 0, 0, 0));
	}

	$effect(() => {
		if (isEmbed) return;
		breadcrumbMeta.value = todayMonthLabel;
		return () => {
			breadcrumbMeta.value = '';
		};
	});

	$effect(() => {
		if (isEmbed) return;
		layoutState.miniMonth;
		loadMiniMonthEvents();
	});

	const calendarSources = $derived(layoutState.calendarSources(text.work, calendarVisibility.work));

	onMount(() => {
		if (isEmbed) return;
		loadSyncInformation();
		loadAccountStatus();
		const storedState = loadCalendarLayoutStoredState();
		if (storedState.visibleDate) {
			layoutState.applyVisibleDate(storedState.visibleDate);
		} else {
			layoutState.selectedMiniDateKey = initialSelectedDateKey(layoutState.today, null);
		}
		if (storedState.view) layoutState.miniMonthCalendarView = storedState.view;
		if (storedState.isWorkVisible !== null) calendarVisibility.work = storedState.isWorkVisible;
		return installCalendarLayoutMessageSync({
			selectedDateKey: () => layoutState.selectedMiniDateKey,
			applyVisibleDate: (date) => layoutState.applyVisibleDate(date),
			applyCalendarView: (view) => {
				layoutState.miniMonthCalendarView = view;
			},
			reloadMiniMonthEvents: loadMiniMonthEvents
		});
	});

	function toggleCalendarSource(sourceID: string) {
		if (sourceID !== 'work') return;
		calendarVisibility.work = !calendarVisibility.work;
		saveAndBroadcastCalendarWorkVisibility(calendarVisibility.work);
	}

	async function loadSyncInformation() {
		try {
			layoutState.syncInformation = await fetchCalendarSyncInformation();
		} catch {
			layoutState.syncInformation = null;
		}
	}

	async function loadAccountStatus() {
		layoutState.isLoadingAccountStatus = true;
		layoutState.accountStatusError = false;
		try {
			layoutState.accountStatus = await fetchCalendarAccountStatus();
		} catch {
			layoutState.accountStatus = null;
			layoutState.accountStatusError = true;
		} finally {
			layoutState.isLoadingAccountStatus = false;
		}
	}

	async function loadMiniMonthEvents() {
		try {
			const events = await fetchCalendarEventsForMonth(layoutState.miniMonth);
			layoutState.setMiniMonthEvents(calendarEventDateKeysForMonth(events, layoutState.miniMonth), events.length);
		} catch {
			layoutState.resetMiniMonthEvents();
		}
	}

	async function rotateSubscriptionURL() {
		layoutState.isRotatingSync = true;
		layoutState.syncError = '';
		try {
			layoutState.syncInformation = await rotateCalendarSubscriptionURL(text.saveError);
		} catch (error) {
			layoutState.syncError = error instanceof Error ? error.message : text.saveError;
		} finally {
			layoutState.isRotatingSync = false;
		}
	}

	async function uploadGoogleOAuthClientFile(file: File): Promise<boolean> {
		layoutState.isUploadingGoogleOAuthClient = true;
		layoutState.syncError = '';
		try {
			await uploadGoogleOAuthClient(file, text.googleOAuthClientUploadError);
			await loadAccountStatus();
			return true;
		} catch (error) {
			layoutState.syncError = error instanceof Error ? error.message : text.googleOAuthClientUploadError;
			return false;
		} finally {
			layoutState.isUploadingGoogleOAuthClient = false;
		}
	}

	function openSyncSheet() {
		layoutState.openSyncSheet();
		loadSyncInformation();
		loadAccountStatus();
	}

	function setMiniMonth(month: Date) {
		layoutState.setMiniMonth(month);
	}
</script>

{#if isEmbed}
	{@render children()}
{:else}
	<CalendarLayoutSidebar
		{text}
		{calendarSources}
		miniMonth={layoutState.miniMonth}
		miniMonthCalendarView={layoutState.miniMonthCalendarView}
		miniMonthEventDates={layoutState.miniMonthEventDates}
		selectedMiniDateKey={layoutState.selectedMiniDateKey}
		{setMiniMonth}
		{selectMiniMonthDate}
		{openSyncSheet}
		refreshCalendar={bumpCalendarRefresh}
		{toggleCalendarSource}
	/>

	<div class="flex min-w-0 flex-1 flex-col">
		{@render children()}
	</div>

	<CalendarLayoutSyncSheet
		bind:isOpen={layoutState.isSyncSheetOpen}
		{text}
		syncInformation={layoutState.syncInformation}
		accountStatus={layoutState.accountStatus}
		accountStatusError={layoutState.accountStatusError}
		isLoadingAccountStatus={layoutState.isLoadingAccountStatus}
		isRotatingSync={layoutState.isRotatingSync}
		isUploadingGoogleOAuthClient={layoutState.isUploadingGoogleOAuthClient}
		syncError={layoutState.syncError}
		{rotateSubscriptionURL}
		uploadGoogleOAuthClient={uploadGoogleOAuthClientFile}
	/>
{/if}
