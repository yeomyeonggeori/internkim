<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
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
		syncError,
		rotateSubscriptionURL
	}: {
		isOpen: boolean;
		text: CalendarSyncSheetText;
		syncInformation: CalendarSyncResponse | null;
		accountStatus: CalendarAccountStatusResponse | null;
		accountStatusError: boolean;
		isLoadingAccountStatus: boolean;
		isRotatingSync: boolean;
		syncError: string;
		rotateSubscriptionURL: () => void;
	} = $props();

	function accountStatusLabel() {
		if (isLoadingAccountStatus) return text.accountStatusLoading;
		if (accountStatusError) return text.accountStatusLoadFailed;
		if (!accountStatus?.connected) return text.googleCalendarDisconnected;
		if (accountStatus.needsReauth) return text.googleCalendarReauthRequired;
		if (accountStatus.accountEmail) {
			return text.googleCalendarConnectedTemplate.replace('{email}', accountStatus.accountEmail);
		}
		return text.googleCalendarConnected;
	}

	function shouldShowGoogleOAuthAction() {
		if (isLoadingAccountStatus || accountStatusError) return false;
		return !accountStatus?.connected || accountStatus.needsReauth;
	}

	function googleOAuthActionLabel() {
		if (accountStatus?.needsReauth) return text.googleCalendarReconnectAction;
		return text.googleCalendarConnectAction;
	}

	function accountStatusDotClass() {
		if (accountStatusError || accountStatus?.needsReauth) return 'bg-warning';
		if (accountStatus?.connected) return 'bg-success';
		return 'bg-muted-foreground/50';
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

			<div class="space-y-2 rounded-md border p-3">
				<p class="text-xs font-medium uppercase text-muted-foreground">{text.externalCalendarAccount}</p>
				<div class="flex items-center gap-2 text-sm">
					<span class={`size-2 rounded-full ${accountStatusDotClass()}`} aria-hidden="true"></span>
					<span class="min-w-0 flex-1 truncate">{accountStatusLabel()}</span>
				</div>
				{#if accountStatus?.needsReauth}
					<p class="text-xs leading-relaxed text-muted-foreground">{text.googleCalendarReconnectHint}</p>
				{/if}
				{#if shouldShowGoogleOAuthAction()}
					<Button href="/calendar/oauth/google/start" variant="outline" class="w-full justify-center gap-2">
						<RefreshCwIcon class="size-4" />
						<span>{googleOAuthActionLabel()}</span>
					</Button>
				{/if}
			</div>

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
