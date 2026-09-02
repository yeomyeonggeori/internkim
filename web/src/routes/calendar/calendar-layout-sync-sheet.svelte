<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import type { CalendarSyncResponse } from './calendar-layout-types';
	import type { CalendarLocaleText } from './text';

	type CalendarSyncSheetText = Pick<
		CalendarLocaleText,
		| 'saveError'
		| 'syncTitle'
		| 'syncDescription'
		| 'subscriptionReady'
		| 'subscriptionShownOnce'
		| 'caldav'
		| 'username'
		| 'password'
		| 'ics'
		| 'rotate'
	>;

	let {
		isOpen = $bindable(false),
		text,
		syncInformation,
		isRotatingSync,
		syncError,
		rotateSubscriptionURL
	}: {
		isOpen: boolean;
		text: CalendarSyncSheetText;
		syncInformation: CalendarSyncResponse | null;
		isRotatingSync: boolean;
		syncError: string;
		rotateSubscriptionURL: () => void;
	} = $props();

	function hasSubscription(): boolean {
		return Boolean(syncInformation?.caldavURL || syncInformation?.icsURL || syncInformation?.isRegistered);
	}
</script>

<Sheet.Root bind:open={isOpen}>
	<Sheet.Content class="w-full sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>{text.syncTitle}</Sheet.Title>
			<Sheet.Description>{text.syncDescription}</Sheet.Description>
		</Sheet.Header>

		<div class="grid min-h-0 flex-1 gap-5 overflow-y-auto px-4 pb-4">
			{#if syncError}
				<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{syncError}</p>
			{/if}

			{#if hasSubscription()}
				<p class="text-xs font-medium text-muted-foreground">{text.subscriptionReady}</p>
			{/if}

			{#if syncInformation?.caldavURL}
				<div class="space-y-3">
					<p class="text-xs font-medium uppercase text-muted-foreground">{text.caldav}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation.caldavURL}
						</code>
						<CopyButton text={syncInformation.caldavURL} variant="outline" size="icon" class="shrink-0" />
					</div>
					<div class="space-y-1">
						<p class="text-[11px] font-medium text-muted-foreground">{text.username}</p>
						<div class="flex min-w-0 items-start gap-2">
							<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
								{syncInformation.caldavUsername ?? ''}
							</code>
							<CopyButton
								text={syncInformation.caldavUsername ?? ''}
								variant="outline"
								size="icon"
								class="shrink-0"
								disabled={!syncInformation.caldavUsername}
							/>
						</div>
					</div>
					<div class="space-y-1">
						<p class="text-[11px] font-medium text-muted-foreground">{text.password}</p>
						<div class="flex min-w-0 items-start gap-2">
							<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
								{syncInformation.caldavPassword ?? ''}
							</code>
							<CopyButton
								text={syncInformation.caldavPassword ?? ''}
								variant="outline"
								size="icon"
								class="shrink-0"
								disabled={!syncInformation.caldavPassword}
							/>
						</div>
					</div>
				</div>

				<Separator />
			{/if}

			{#if syncInformation?.icsURL}
				<div class="space-y-2">
					<p class="text-xs font-medium uppercase text-muted-foreground">{text.ics}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation.icsURL}
						</code>
						<CopyButton text={syncInformation.icsURL} variant="outline" size="icon" class="shrink-0" />
					</div>
				</div>
			{/if}

			{#if syncInformation?.isRegistered && !syncInformation?.icsURL}
				<p class="text-xs text-muted-foreground">{text.subscriptionShownOnce}</p>
			{/if}

			{#if hasSubscription()}
				<Button variant="outline" class="w-full justify-center gap-2" onclick={rotateSubscriptionURL} disabled={isRotatingSync}>
					<RotateCwIcon class={isRotatingSync ? 'size-4 animate-spin' : 'size-4'} />
					<span>{text.rotate}</span>
				</Button>
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
