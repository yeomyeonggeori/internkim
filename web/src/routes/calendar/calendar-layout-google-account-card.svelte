<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import {
		googleOAuthActionLabel,
		googleOAuthStartURLForReturnURL,
		shouldShowGoogleCalendarSelector,
		shouldShowGoogleOAuthAction,
		shouldShowGoogleOAuthInitialUpload,
		shouldShowGoogleOAuthReadyHint,
		shouldShowGoogleOAuthReplacementUpload,
		shouldShowGoogleOAuthUnavailableHint
	} from './calendar-google-account-state';
	import { openGoogleOAuthWindow } from './calendar-google-oauth-window';
	import GoogleAccountStatus from './calendar-google-account-status.svelte';
	import CalendarGoogleCalendarSelector from './calendar-google-calendar-selector.svelte';
	import GoogleOAuthClientUploadPanel from './calendar-google-oauth-client-upload-panel.svelte';
	import type { CalendarAccountStatusResponse, GoogleCalendarListEntry } from './calendar-layout-types';
	import type { CalendarGoogleAccountText } from './text';

	let {
		text,
		accountStatus,
		accountStatusError,
		isLoadingAccountStatus,
		googleCalendars,
		isLoadingGoogleCalendars,
		isSelectingGoogleCalendar,
		isUploadingGoogleOAuthClient,
		googleCalendarSelectionError,
		selectGoogleCalendar,
		uploadGoogleOAuthClient
	}: {
		text: CalendarGoogleAccountText;
		accountStatus: CalendarAccountStatusResponse | null;
		accountStatusError: boolean;
		isLoadingAccountStatus: boolean;
		googleCalendars: GoogleCalendarListEntry[];
		isLoadingGoogleCalendars: boolean;
		isSelectingGoogleCalendar: boolean;
		isUploadingGoogleOAuthClient: boolean;
		googleCalendarSelectionError: string;
		selectGoogleCalendar: (calendarID: string) => Promise<boolean>;
		uploadGoogleOAuthClient: (file: File) => Promise<boolean>;
	} = $props();

	const googleOAuthJavascriptOrigin = $derived(page.url.origin);
	const googleOAuthRedirectURI = $derived(`${googleOAuthJavascriptOrigin}/calendar/oauth/google/callback`);
	const googleOAuthReturnURL = $derived(`${page.url.origin}/calendar/`);
	const googleOAuthStartURL = $derived(googleOAuthStartURLForReturnURL(googleOAuthReturnURL));

	function handleGoogleOAuthActionClick(event: MouseEvent) {
		event.preventDefault();
		openGoogleOAuthWindow(googleOAuthStartURL);
	}
</script>

<div class="space-y-2 rounded-md border p-3">
	<GoogleAccountStatus
		{text}
		{accountStatus}
		{accountStatusError}
		{isLoadingAccountStatus}
		googleOAuthSwitchAccountURL={googleOAuthStartURL}
	/>
	{#if accountStatus?.needsReauth && accountStatus.googleOAuthConfigured}
		<p class="text-xs leading-relaxed text-muted-foreground">{text.googleCalendarReconnectHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthReadyHint(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<p class="text-xs leading-relaxed text-muted-foreground">{text.googleCalendarReadyHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthUnavailableHint(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<p class="whitespace-pre-line text-xs leading-relaxed text-muted-foreground">{text.googleCalendarUnavailableHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthInitialUpload(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<GoogleOAuthClientUploadPanel
			{text}
			{isUploadingGoogleOAuthClient}
			{googleOAuthJavascriptOrigin}
			{googleOAuthRedirectURI}
			{uploadGoogleOAuthClient}
		/>
	{/if}
	{#if shouldShowGoogleOAuthAction(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<Button
			href={googleOAuthStartURL}
			target="_blank"
			rel="noopener noreferrer"
			variant="outline"
			class="w-full justify-center gap-2"
			onclick={handleGoogleOAuthActionClick}
		>
			<RefreshCwIcon class="size-4" />
			<span>{googleOAuthActionLabel(text, accountStatus)}</span>
		</Button>
	{/if}
	{#if shouldShowGoogleCalendarSelector(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<CalendarGoogleCalendarSelector
			{text}
			calendars={googleCalendars}
			selectedCalendarID={accountStatus?.selectedCalendarID ?? ''}
			{isLoadingGoogleCalendars}
			{isSelectingGoogleCalendar}
			selectionError={googleCalendarSelectionError}
			{selectGoogleCalendar}
		/>
	{/if}
	{#if shouldShowGoogleOAuthReplacementUpload(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<details class="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
			<summary class="cursor-pointer text-sm font-medium text-foreground">{text.googleOAuthClientReplaceTitle}</summary>
			<p class="mt-2 leading-relaxed">{text.googleOAuthClientReplaceHint}</p>
			<div class="mt-3">
				<GoogleOAuthClientUploadPanel
					{text}
					{isUploadingGoogleOAuthClient}
					{googleOAuthJavascriptOrigin}
					{googleOAuthRedirectURI}
					{uploadGoogleOAuthClient}
				/>
			</div>
		</details>
	{/if}
</div>
