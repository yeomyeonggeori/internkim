<script lang="ts">
	import { page } from '$app/state';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { onMount } from 'svelte';
	import {
		fetchCalendarAccountStatus,
		fetchCalendarSyncInformation,
		rotateCalendarSubscriptionURL,
		uploadGoogleOAuthClient
	} from './calendar-layout-api';
	import {
		installCalendarLayoutMessageSync,
		loadCalendarLayoutStoredState
	} from './calendar-layout-message-sync';
	import { CalendarLayoutState } from './calendar-layout-state.svelte';
	import CalendarLayoutSyncSheet from './calendar-layout-sync-sheet.svelte';
	import { bumpCalendarRefresh } from './refresh-signal.svelte';
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

	$effect(() => {
		if (isEmbed) return;
		breadcrumbMeta.value = todayMonthLabel;
		return () => {
			breadcrumbMeta.value = '';
		};
	});

	onMount(() => {
		if (isEmbed) return;
		loadSyncInformation();
		loadAccountStatus();
		const storedState = loadCalendarLayoutStoredState();
		if (storedState.visibleDate) {
			layoutState.applyVisibleDate(storedState.visibleDate);
		}
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
