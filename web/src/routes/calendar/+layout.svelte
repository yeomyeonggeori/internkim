<script lang="ts">
	import { page } from '$app/state';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { onMount } from 'svelte';
	import {
		fetchCalendarAccountStatus,
		fetchGoogleCalendarList,
		fetchCalendarSyncInformation,
		rotateCalendarSubscriptionURL,
		saveGoogleCalendarSelection,
		uploadGoogleOAuthClient
	} from './calendar-layout-api';
	import {
		installCalendarLayoutMessageSync,
		loadCalendarLayoutStoredState
	} from './calendar-layout-message-sync';
	import {
		googleOAuthReturnStatusFromMessageEvent,
		googleOAuthReturnStatusFromStorageEvent,
		googleOAuthReturnStatusFromURL,
		notifyGoogleOAuthReturn,
		type GoogleOAuthReturnStatus
	} from './calendar-google-oauth-return';
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
		handleGoogleOAuthReturn();
		loadSyncInformation();
		loadAccountStatus();
		const storedState = loadCalendarLayoutStoredState();
		if (storedState.visibleDate) {
			layoutState.applyVisibleDate(storedState.visibleDate);
		}
		window.addEventListener('message', handleGoogleOAuthMessage);
		window.addEventListener('storage', handleGoogleOAuthStorageEvent);
		const uninstallMessageSync = installCalendarLayoutMessageSync({
			selectedDateKey: () => layoutState.selectedDateKey,
			applyVisibleDate: (date) => layoutState.applyVisibleDate(date),
			openSettings: openSyncSheet,
			refreshCalendar: bumpCalendarRefresh
		});
		return () => {
			window.removeEventListener('message', handleGoogleOAuthMessage);
			window.removeEventListener('storage', handleGoogleOAuthStorageEvent);
			uninstallMessageSync();
		};
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
			const accountStatus = await fetchCalendarAccountStatus();
			layoutState.accountStatus = accountStatus;
			if (shouldLoadGoogleCalendars(accountStatus)) {
				await loadGoogleCalendars();
			} else {
				clearGoogleCalendars();
			}
		} catch {
			layoutState.accountStatus = null;
			layoutState.accountStatusError = true;
			clearGoogleCalendars();
		} finally {
			layoutState.isLoadingAccountStatus = false;
		}
	}

	async function loadGoogleCalendars() {
		layoutState.isLoadingGoogleCalendars = true;
		layoutState.googleCalendarSelectionError = '';
		try {
			const response = await fetchGoogleCalendarList(text.googleCalendarListLoadError);
			layoutState.googleCalendars = response.calendars;
		} catch (error) {
			layoutState.googleCalendars = [];
			layoutState.googleCalendarSelectionError = error instanceof Error ? error.message : text.googleCalendarListLoadError;
			await refreshAccountStatusAfterGoogleCalendarFailure();
		} finally {
			layoutState.isLoadingGoogleCalendars = false;
		}
	}

	async function refreshAccountStatusAfterGoogleCalendarFailure() {
		try {
			const accountStatus = await fetchCalendarAccountStatus();
			layoutState.accountStatus = accountStatus;
			layoutState.accountStatusError = false;
			if (!shouldLoadGoogleCalendars(accountStatus)) {
				clearGoogleCalendars();
			}
		} catch {
			layoutState.accountStatusError = true;
		}
	}

	function clearGoogleCalendars() {
		layoutState.googleCalendars = [];
		layoutState.isLoadingGoogleCalendars = false;
		layoutState.googleCalendarSelectionError = '';
	}

	function shouldLoadGoogleCalendars(accountStatus: Awaited<ReturnType<typeof fetchCalendarAccountStatus>>) {
		return (
			accountStatus.connected &&
			accountStatus.googleOAuthConfigured &&
			accountStatus.canManageGoogleOAuth &&
			!accountStatus.needsReauth
		);
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

	async function selectGoogleCalendar(calendarID: string): Promise<boolean> {
		layoutState.isSelectingGoogleCalendar = true;
		layoutState.googleCalendarSelectionError = '';
		try {
			await saveGoogleCalendarSelection(calendarID, text.googleCalendarSelectionSaveError);
			await loadAccountStatus();
			await loadSyncInformation();
			bumpCalendarRefresh();
			return true;
		} catch (error) {
			layoutState.googleCalendarSelectionError =
				error instanceof Error ? error.message : text.googleCalendarSelectionSaveError;
			await refreshAccountStatusAfterGoogleCalendarFailure();
			return false;
		} finally {
			layoutState.isSelectingGoogleCalendar = false;
		}
	}

	async function uploadGoogleOAuthClientFile(file: File): Promise<boolean> {
		layoutState.isUploadingGoogleOAuthClient = true;
		layoutState.syncError = '';
		layoutState.syncNotice = '';
		try {
			await uploadGoogleOAuthClient(file, text.googleOAuthClientUploadError);
			await loadAccountStatus();
			layoutState.syncNotice = 'googleOAuthClientUploaded';
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

	function handleGoogleOAuthReturn() {
		const returnStatus = googleOAuthReturnStatusFromURL(page.url);
		if (!returnStatus) return;
		notifyGoogleOAuthReturn(returnStatus);
		showGoogleOAuthReturnNotice(returnStatus);
		clearGoogleOAuthReturnQuery();
	}

	function handleGoogleOAuthStorageEvent(event: StorageEvent) {
		if (isEmbed) return;
		const returnStatus = googleOAuthReturnStatusFromStorageEvent(event);
		if (!returnStatus) return;
		showGoogleOAuthReturnNotice(returnStatus);
	}

	function handleGoogleOAuthMessage(event: MessageEvent) {
		if (isEmbed) return;
		const returnStatus = googleOAuthReturnStatusFromMessageEvent(event, window.location.origin);
		if (!returnStatus) return;
		showGoogleOAuthReturnNotice(returnStatus);
	}

	function showGoogleOAuthReturnNotice(returnStatus: GoogleOAuthReturnStatus) {
		layoutState.syncNotice = returnStatus === 'connected' ? 'googleOAuthConnected' : 'googleOAuthFailed';
		openSyncSheet();
	}

	function clearGoogleOAuthReturnQuery() {
		const nextURL = new URL(window.location.href);
		nextURL.searchParams.delete('googleOAuth');
		window.history.replaceState(window.history.state, '', `${nextURL.pathname}${nextURL.search}${nextURL.hash}`);
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
		googleCalendars={layoutState.googleCalendars}
		accountStatusError={layoutState.accountStatusError}
		isLoadingAccountStatus={layoutState.isLoadingAccountStatus}
		isLoadingGoogleCalendars={layoutState.isLoadingGoogleCalendars}
		isSelectingGoogleCalendar={layoutState.isSelectingGoogleCalendar}
		isRotatingSync={layoutState.isRotatingSync}
		isUploadingGoogleOAuthClient={layoutState.isUploadingGoogleOAuthClient}
		syncError={layoutState.syncError}
		syncNotice={layoutState.syncNotice}
		googleCalendarSelectionError={layoutState.googleCalendarSelectionError}
		{rotateSubscriptionURL}
		{selectGoogleCalendar}
		uploadGoogleOAuthClient={uploadGoogleOAuthClientFile}
	/>
{/if}
