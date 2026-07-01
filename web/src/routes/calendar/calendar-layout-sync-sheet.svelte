<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import CalendarLayoutGoogleAccountCard from './calendar-layout-google-account-card.svelte';
	import type {
		CalendarAccountStatusResponse,
		CalendarSyncResponse
	} from './calendar-layout-types';

	type CalendarSyncSheetText = {
		accountStatusLoading: string;
		accountStatusLoadFailed: string;
		googleCalendarDisconnected: string;
		googleCalendarConnected: string;
		googleCalendarConnectedTemplate: string;
		googleCalendarReauthRequired: string;
		googleCalendarConnectAction: string;
		googleCalendarReconnectAction: string;
		googleCalendarReconnectHint: string;
		googleCalendarReadyHint: string;
		googleCalendarUnavailableHint: string;
		googleOAuthClientUploadTitle: string;
		googleOAuthClientUploadHint: string;
		googleOAuthClientUploadAction: string;
		googleOAuthClientUploading: string;
		googleOAuthClientFileLabel: string;
		googleOAuthClientChooseFile: string;
		googleOAuthClientReplaceTitle: string;
		googleOAuthClientReplaceHint: string;
		googleOAuthClientGuide: {
			title: string;
			intro: string;
			checklistTitle: string;
			checks: string[];
			stepsTitle: string;
			steps: Array<{
				title: string;
				body: string;
				action?: {
					label: string;
					url: string;
				};
			}>;
			redirectURI: string;
			javascriptOrigin: string;
		};
		saveError: string;
		syncTitle: string;
		syncDescription: string;
		externalCalendarAccount: string;
		subscriptionReady: string;
		caldav: string;
		username: string;
		password: string;
		ics: string;
		rotate: string;
	};

	let {
		isOpen = $bindable(false),
		text,
		syncInformation,
		accountStatus,
		accountStatusError,
		isLoadingAccountStatus,
		isRotatingSync,
		isUploadingGoogleOAuthClient,
		syncError,
		rotateSubscriptionURL,
		uploadGoogleOAuthClient
	}: {
		isOpen: boolean;
		text: CalendarSyncSheetText;
		syncInformation: CalendarSyncResponse | null;
		accountStatus: CalendarAccountStatusResponse | null;
		accountStatusError: boolean;
		isLoadingAccountStatus: boolean;
		isRotatingSync: boolean;
		isUploadingGoogleOAuthClient: boolean;
		syncError: string;
		rotateSubscriptionURL: () => void;
		uploadGoogleOAuthClient: (file: File) => Promise<boolean>;
	} = $props();

	function canManageGoogleOAuth(): boolean {
		if (isLoadingAccountStatus || accountStatusError) return false;
		return accountStatus?.canManageGoogleOAuth === true;
	}
</script>

<Sheet.Root bind:open={isOpen}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>{text.syncTitle}</Sheet.Title>
			<Sheet.Description>{text.syncDescription}</Sheet.Description>
		</Sheet.Header>

		<div class="grid gap-5 px-4 pb-4">
			{#if syncError}
				<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{syncError}</p>
			{/if}

			{#if canManageGoogleOAuth()}
				<CalendarLayoutGoogleAccountCard
					{text}
					{accountStatus}
					{accountStatusError}
					{isLoadingAccountStatus}
					{isUploadingGoogleOAuthClient}
					{uploadGoogleOAuthClient}
				/>
			{/if}

			<div class="space-y-3">
				<p class="text-xs font-medium text-muted-foreground">{text.subscriptionReady}</p>
				<p class="text-xs font-medium uppercase text-muted-foreground">{text.caldav}</p>
				<div class="flex min-w-0 items-start gap-2">
					<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
						{syncInformation?.caldavURL ?? ''}
					</code>
					<CopyButton text={syncInformation?.caldavURL ?? ''} variant="outline" size="icon" class="shrink-0" disabled={!syncInformation?.caldavURL} />
				</div>
				<div class="space-y-1">
					<p class="text-[11px] font-medium text-muted-foreground">{text.username}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation?.caldavUsername ?? ''}
						</code>
						<CopyButton
							text={syncInformation?.caldavUsername ?? ''}
							variant="outline"
							size="icon"
							class="shrink-0"
							disabled={!syncInformation?.caldavUsername}
						/>
					</div>
				</div>
				<div class="space-y-1">
					<p class="text-[11px] font-medium text-muted-foreground">{text.password}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation?.caldavPassword ?? ''}
						</code>
						<CopyButton
							text={syncInformation?.caldavPassword ?? ''}
							variant="outline"
							size="icon"
							class="shrink-0"
							disabled={!syncInformation?.caldavPassword}
						/>
					</div>
				</div>
			</div>

			<Separator />

			<div class="space-y-2">
				<p class="text-xs font-medium uppercase text-muted-foreground">{text.ics}</p>
				<div class="flex min-w-0 items-start gap-2">
					<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
						{syncInformation?.icsURL ?? ''}
					</code>
					<CopyButton text={syncInformation?.icsURL ?? ''} variant="outline" size="icon" class="shrink-0" disabled={!syncInformation?.icsURL} />
				</div>
			</div>

			<Button variant="outline" class="w-full justify-center gap-2" onclick={rotateSubscriptionURL} disabled={isRotatingSync}>
				<RotateCwIcon class={isRotatingSync ? 'size-4 animate-spin' : 'size-4'} />
				<span>{text.rotate}</span>
			</Button>
		</div>
	</Sheet.Content>
</Sheet.Root>
