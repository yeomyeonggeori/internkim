<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import {
		googleAccountStatusLabel,
		selectedGoogleCalendarLabel,
		shouldShowGoogleOAuthSwitchAccountAction
	} from './calendar-google-account-state';
	import {
		googleCalendarConnectionStateClass,
		googleCalendarConnectionStateLabel
	} from './calendar-google-calendar-readiness-state';
	import { openGoogleOAuthWindow } from './calendar-google-oauth-window';
	import type { CalendarAccountStatusResponse } from './calendar-layout-types';
	import type { CalendarGoogleAccountText } from './text';

	let {
		text,
		accountStatus,
		accountStatusError,
		isLoadingAccountStatus,
		googleOAuthSwitchAccountURL
	}: {
		text: CalendarGoogleAccountText;
		accountStatus: CalendarAccountStatusResponse | null;
		accountStatusError: boolean;
		isLoadingAccountStatus: boolean;
		googleOAuthSwitchAccountURL: string;
	} = $props();

	const selectedCalendarLabel = $derived(selectedGoogleCalendarLabel(accountStatus));
	const connectionStateLabel = $derived(
		googleCalendarConnectionStateLabel(text, accountStatus, accountStatusError, isLoadingAccountStatus)
	);

	function handleGoogleOAuthSwitchAccountClick(event: MouseEvent) {
		event.preventDefault();
		openGoogleOAuthWindow(googleOAuthSwitchAccountURL);
	}
</script>

<div class="flex min-w-0 items-start justify-between gap-2">
	<p class="min-w-0 text-xs font-medium uppercase text-muted-foreground">{text.externalCalendarAccount}</p>
	{#if shouldShowGoogleOAuthSwitchAccountAction(accountStatus, accountStatusError, isLoadingAccountStatus)}
		<Button
			href={googleOAuthSwitchAccountURL}
			target="_blank"
			rel="noopener noreferrer"
			variant="outline"
			size="sm"
			class="shrink-0 gap-1 px-2 text-xs"
			onclick={handleGoogleOAuthSwitchAccountClick}
		>
			<RefreshCwIcon class="size-3.5" />
			<span>{text.googleCalendarSwitchAccountAction}</span>
		</Button>
	{/if}
</div>
<p class="min-w-0 truncate text-sm font-medium">
	{googleAccountStatusLabel(text, accountStatus, accountStatusError, isLoadingAccountStatus)}
</p>
{#if selectedCalendarLabel}
	<p class="min-w-0 truncate text-xs text-muted-foreground">
		{text.googleCalendarSelectedCalendarTemplate.replace('{calendar}', selectedCalendarLabel)}
	</p>
{/if}
{#if connectionStateLabel}
	<p class={`inline-flex max-w-full rounded-md border px-2 py-1 text-xs font-medium ${googleCalendarConnectionStateClass(accountStatus)}`}>
		<span class="min-w-0 truncate">{connectionStateLabel}</span>
	</p>
{/if}
