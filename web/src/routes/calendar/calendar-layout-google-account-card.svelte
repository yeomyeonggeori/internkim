<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import CalendarGoogleCalendarSelector from './calendar-google-calendar-selector.svelte';
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

	let selectedGoogleOAuthClientFile = $state<File | null>(null);
	let isGoogleOAuthClientDropActive = $state(false);
	const googleOAuthJavascriptOrigin = 'https://{본인 서버 URL}';
	const googleOAuthRedirectURI = `${googleOAuthJavascriptOrigin}/calendar/oauth/google/callback`;
	const googleOAuthReturnURL = $derived(`${page.url.origin}/calendar/`);
	const googleOAuthStartURL = $derived(`/calendar/oauth/google/start?returnTo=${encodeURIComponent(googleOAuthReturnURL)}`);

	function isWaitingForInitialAccountStatus() {
		return isLoadingAccountStatus && !accountStatus;
	}

	function accountStatusLabel() {
		if (isWaitingForInitialAccountStatus()) return text.accountStatusLoading;
		if (accountStatusError) return text.accountStatusLoadFailed;
		if (!accountStatus?.connected) return text.googleCalendarDisconnected;
		if (accountStatus.needsReauth) return text.googleCalendarReauthRequired;
		if (accountStatus.accountEmail) {
			return text.googleCalendarConnectedTemplate.replace('{email}', accountStatus.accountEmail);
		}
		return text.googleCalendarConnected;
	}

	function shouldShowGoogleOAuthAction() {
		if (isWaitingForInitialAccountStatus() || accountStatusError) return false;
		if (accountStatus?.googleOAuthConfigured !== true) return false;
		return !accountStatus.connected || accountStatus.needsReauth;
	}

	function shouldShowGoogleOAuthUnavailableHint() {
		if (isWaitingForInitialAccountStatus() || accountStatusError) return false;
		if (accountStatus?.googleOAuthConfigured !== false) return false;
		return !accountStatus.connected || accountStatus.needsReauth;
	}

	function shouldShowGoogleOAuthReadyHint() {
		if (isWaitingForInitialAccountStatus() || accountStatusError) return false;
		if (accountStatus?.googleOAuthConfigured !== true) return false;
		return !accountStatus.connected && !accountStatus.needsReauth;
	}

	function canManageGoogleOAuth() {
		if (isWaitingForInitialAccountStatus() || accountStatusError) return false;
		return accountStatus?.canManageGoogleOAuth === true;
	}

	function shouldShowGoogleOAuthInitialUpload() {
		if (!canManageGoogleOAuth()) return false;
		return accountStatus?.googleOAuthConfigured === false;
	}

	function shouldShowGoogleOAuthReplacementUpload() {
		if (!canManageGoogleOAuth()) return false;
		return accountStatus?.googleOAuthConfigured === true;
	}

	function shouldShowGoogleCalendarSelector() {
		if (!canManageGoogleOAuth()) return false;
		if (!accountStatus?.connected || accountStatus.needsReauth) return false;
		return accountStatus.googleOAuthConfigured === true;
	}

	function googleOAuthActionLabel() {
		if (accountStatus?.needsReauth) return text.googleCalendarReconnectAction;
		return text.googleCalendarConnectAction;
	}

	function selectedGoogleCalendarLabel() {
		return accountStatus?.selectedCalendarName || accountStatus?.selectedCalendarID || '';
	}

	function hasWritableSelectedCalendar() {
		const accessRole = accountStatus?.selectedCalendarAccessRole?.trim().toLowerCase();
		return accessRole === 'writer' || accessRole === 'owner';
	}

	function calendarConnectionStateLabel() {
		if (isWaitingForInitialAccountStatus() || accountStatusError || !accountStatus?.connected) return '';
		if (accountStatus.needsReauth) return text.googleCalendarReauthRequired;
		if (accountStatus.needsCalendarSelection) return text.googleCalendarSelectionRequired;
		if (!hasWritableSelectedCalendar()) return text.googleCalendarWritePermissionRequired;
		if (!accountStatus.initialSyncCompleted) return text.googleCalendarInitialSyncPending;
		if (accountStatus.calendarSyncReady) return text.googleCalendarSyncReady;
		return text.googleCalendarInitialSyncPending;
	}

	function calendarConnectionStateClass() {
		if (!accountStatus?.connected || accountStatus.needsCalendarSelection || !accountStatus.initialSyncCompleted) {
			return 'border-border bg-muted text-muted-foreground';
		}
		if (accountStatus.calendarSyncReady) {
			return 'border-emerald-200 bg-emerald-50 text-emerald-700';
		}
		return 'border-destructive/30 bg-destructive/10 text-destructive';
	}

	function selectGoogleOAuthClientFile(fileList: FileList | null) {
		selectedGoogleOAuthClientFile = fileList?.[0] ?? null;
	}

	function handleGoogleOAuthClientDrop(event: DragEvent) {
		event.preventDefault();
		isGoogleOAuthClientDropActive = false;
		selectGoogleOAuthClientFile(event.dataTransfer?.files ?? null);
	}

	function handleGoogleOAuthClientDragOver(event: DragEvent) {
		event.preventDefault();
		isGoogleOAuthClientDropActive = true;
	}

	function handleGoogleOAuthClientDragLeave() {
		isGoogleOAuthClientDropActive = false;
	}

	async function submitGoogleOAuthClientFile() {
		if (!selectedGoogleOAuthClientFile || isUploadingGoogleOAuthClient) return;
		const didUploadGoogleOAuthClient = await uploadGoogleOAuthClient(selectedGoogleOAuthClientFile);
		if (didUploadGoogleOAuthClient) {
			selectedGoogleOAuthClientFile = null;
		}
	}
</script>

{#snippet googleOAuthClientUploadPanel()}
	<div
		role="group"
		aria-label={text.googleOAuthClientUploadTitle}
		class={`grid min-w-0 gap-3 rounded-md border border-dashed px-3 py-3 ${isGoogleOAuthClientDropActive ? 'border-primary bg-primary/5' : 'border-border bg-muted/30'}`}
		ondrop={handleGoogleOAuthClientDrop}
		ondragover={handleGoogleOAuthClientDragOver}
		ondragleave={handleGoogleOAuthClientDragLeave}
	>
		<div class="flex min-w-0 items-start gap-2">
			<UploadIcon class="mt-0.5 size-4 shrink-0 text-muted-foreground" />
			<div class="min-w-0 space-y-1">
				<p class="text-sm font-medium">{text.googleOAuthClientUploadTitle}</p>
				<p class="text-xs leading-relaxed text-muted-foreground">{text.googleOAuthClientUploadHint}</p>
				{#if selectedGoogleOAuthClientFile}
					<p class="truncate text-xs text-foreground">{selectedGoogleOAuthClientFile.name}</p>
				{/if}
			</div>
		</div>
		<div class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2">
			<label
				for="google-oauth-client-file"
				class="flex h-9 min-w-0 cursor-pointer items-center justify-center rounded-md border bg-background px-3 text-sm font-medium hover:bg-muted"
			>
				<span class="block min-w-0 truncate">
					{selectedGoogleOAuthClientFile?.name ?? text.googleOAuthClientChooseFile}
				</span>
			</label>
			<input
				id="google-oauth-client-file"
				aria-label={text.googleOAuthClientFileLabel}
				accept="application/json,.json"
				type="file"
				class="sr-only"
				onchange={(event) => selectGoogleOAuthClientFile(event.currentTarget.files)}
			/>
			<Button
				variant="outline"
				class="justify-center gap-2"
				disabled={!selectedGoogleOAuthClientFile || isUploadingGoogleOAuthClient}
				onclick={submitGoogleOAuthClientFile}
			>
				<UploadIcon class={isUploadingGoogleOAuthClient ? 'size-4 animate-pulse' : 'size-4'} />
				<span>{isUploadingGoogleOAuthClient ? text.googleOAuthClientUploading : text.googleOAuthClientUploadAction}</span>
			</Button>
		</div>
		<details class="rounded-md border bg-background px-3 py-2 text-xs text-muted-foreground">
			<summary class="cursor-pointer text-sm font-medium text-foreground">{text.googleOAuthClientGuide.title}</summary>
			<div class="mt-3 space-y-3">
				<p class="leading-relaxed">{text.googleOAuthClientGuide.intro}</p>
				<div class="space-y-1">
					<p class="font-medium text-foreground">{text.googleOAuthClientGuide.checklistTitle}</p>
					<ul class="list-disc space-y-1 pl-4">
						{#each text.googleOAuthClientGuide.checks as check}
							<li>{check}</li>
						{/each}
					</ul>
				</div>
				<div class="space-y-2">
					<p class="font-medium text-foreground">{text.googleOAuthClientGuide.stepsTitle}</p>
					<ol class="list-decimal space-y-2 pl-4">
						{#each text.googleOAuthClientGuide.steps as step}
							<li>
								<p class="font-medium text-foreground">{step.title}</p>
								<p class="mt-0.5 leading-relaxed">{step.body}</p>
								{#if step.action}
									<a
										href={step.action.url}
										target="_blank"
										rel="noreferrer"
										class="mt-1 inline-flex text-xs font-medium text-primary underline-offset-4 hover:underline"
									>
										{step.action.label}
									</a>
								{/if}
							</li>
						{/each}
					</ol>
				</div>
				<div class="space-y-1">
					<p class="font-medium text-foreground">{text.googleOAuthClientGuide.redirectURI}</p>
					<code class="block rounded-md bg-muted px-2 py-1.5 font-mono text-[11px] break-all text-foreground">{googleOAuthRedirectURI}</code>
				</div>
				<div class="space-y-1">
					<p class="font-medium text-foreground">{text.googleOAuthClientGuide.javascriptOrigin}</p>
					<code class="block rounded-md bg-muted px-2 py-1.5 font-mono text-[11px] break-all text-foreground">{googleOAuthJavascriptOrigin}</code>
				</div>
			</div>
		</details>
	</div>
{/snippet}

<div class="space-y-2 rounded-md border p-3">
	<p class="text-xs font-medium uppercase text-muted-foreground">{text.externalCalendarAccount}</p>
	<p class="min-w-0 truncate text-sm font-medium">{accountStatusLabel()}</p>
	{#if selectedGoogleCalendarLabel()}
		<p class="min-w-0 truncate text-xs text-muted-foreground">
			{text.googleCalendarSelectedCalendarTemplate.replace('{calendar}', selectedGoogleCalendarLabel())}
		</p>
	{/if}
	{#if calendarConnectionStateLabel()}
		<p class={`inline-flex max-w-full rounded-md border px-2 py-1 text-xs font-medium ${calendarConnectionStateClass()}`}>
			<span class="min-w-0 truncate">{calendarConnectionStateLabel()}</span>
		</p>
	{/if}
	{#if accountStatus?.needsReauth && accountStatus.googleOAuthConfigured}
		<p class="text-xs leading-relaxed text-muted-foreground">{text.googleCalendarReconnectHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthReadyHint()}
		<p class="text-xs leading-relaxed text-muted-foreground">{text.googleCalendarReadyHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthUnavailableHint()}
		<p class="whitespace-pre-line text-xs leading-relaxed text-muted-foreground">{text.googleCalendarUnavailableHint}</p>
	{/if}
	{#if shouldShowGoogleOAuthInitialUpload()}
		{@render googleOAuthClientUploadPanel()}
	{/if}
	{#if shouldShowGoogleOAuthAction()}
		<Button
			href={googleOAuthStartURL}
			target="_blank"
			rel="noopener noreferrer"
			variant="outline"
			class="w-full justify-center gap-2"
		>
			<RefreshCwIcon class="size-4" />
			<span>{googleOAuthActionLabel()}</span>
		</Button>
	{/if}
	{#if shouldShowGoogleCalendarSelector()}
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
	{#if shouldShowGoogleOAuthReplacementUpload()}
		<details class="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
			<summary class="cursor-pointer text-sm font-medium text-foreground">{text.googleOAuthClientReplaceTitle}</summary>
			<p class="mt-2 leading-relaxed">{text.googleOAuthClientReplaceHint}</p>
			<div class="mt-3">
				{@render googleOAuthClientUploadPanel()}
			</div>
		</details>
	{/if}
</div>
