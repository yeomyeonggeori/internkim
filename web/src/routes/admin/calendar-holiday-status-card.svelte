<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { AdminPageText, CalendarHolidaySyncState, UserRole } from './admin-types';
	import { CalendarHolidayStatusState } from './calendar-holiday-status-state.svelte';

	type CalendarHolidayStatusCardProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		role: UserRole;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, role, text }: CalendarHolidayStatusCardProps = $props();
	const holidayStatusState = new CalendarHolidayStatusState();
	let loadedAdminBaseURL = $state('');
	const canManageHolidays = $derived(role === 'admin' || role === 'operationsAdmin');

	$effect(() => {
		if (!canManageHolidays || !adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		holidayStatusState.load(adminBaseURL, text.holidayStatus.loadError);
	});

	function statusLabel(status: CalendarHolidaySyncState): string {
		return text.holidayStatus[status];
	}

	function displayValue(value?: string): string {
		return value || text.holidayStatus.emptyValue;
	}
</script>

{#if canManageHolidays}
	<Card.Root>
		<Card.Header class="border-b pb-4">
			<Card.Title>{text.holidayStatus.title}</Card.Title>
			<Card.Description>{text.holidayStatus.description}</Card.Description>
			{#if holidayStatusState.status}
				<Card.Action>
					<Badge variant={holidayStatusState.status.status === 'healthy' ? 'secondary' : 'outline'}>{statusLabel(holidayStatusState.status.status)}</Badge>
				</Card.Action>
			{/if}
		</Card.Header>
		<Card.Content class="space-y-4">
			{#if holidayStatusState.isLoading}
				<div class="flex items-center gap-2 text-sm text-muted-foreground"><LoaderIcon class="size-4 animate-spin" />{text.holidayStatus.loading}</div>
			{:else if holidayStatusState.status}
				<div class="grid gap-3 text-sm sm:grid-cols-2">
					<div><span class="text-muted-foreground">{text.holidayStatus.country}</span><div class="font-medium">{holidayStatusState.status.countryCode}</div></div>
					<div><span class="text-muted-foreground">{text.holidayStatus.provider}</span><div class="font-medium">{holidayStatusState.status.provider}</div></div>
				</div>
				{#each holidayStatusState.status.years as year (year.year)}
					<div class="rounded-md border p-4">
						<div class="mb-3 flex items-center justify-between"><span class="font-medium">{text.holidayStatus.year} {year.year}</span><Badge variant={year.status === 'healthy' ? 'secondary' : 'outline'}>{statusLabel(year.status)}</Badge></div>
						<dl class="grid gap-3 text-sm sm:grid-cols-2">
							<div><dt class="text-muted-foreground">{text.holidayStatus.cacheCount}</dt><dd>{year.cacheCount}</dd></div>
							<div><dt class="text-muted-foreground">{text.holidayStatus.lastSyncedAt}</dt><dd>{displayValue(year.lastSyncedAt)}</dd></div>
							<div><dt class="text-muted-foreground">{text.holidayStatus.lastAttemptAt}</dt><dd>{displayValue(year.lastAttemptAt)}</dd></div>
							<div><dt class="text-muted-foreground">{text.holidayStatus.nextRetryAt}</dt><dd>{displayValue(year.nextRetryAt)}</dd></div>
						</dl>
						{#if year.lastError}
							<div class="mt-3"><div class="mb-1 text-sm text-muted-foreground">{text.holidayStatus.actualError}</div><pre class="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-xs">{year.lastError}</pre></div>
						{/if}
					</div>
				{/each}
			{/if}
			{#if holidayStatusState.message}<p class="text-sm text-destructive">{holidayStatusState.message}</p>{/if}
		</Card.Content>
		<Card.Footer class="justify-end">
			<Button variant="outline" disabled={!isDeviceReachable || holidayStatusState.isLoading || holidayStatusState.isRefreshing} onclick={() => holidayStatusState.refresh(adminBaseURL, text.holidayStatus.refreshError)}>
				{#if holidayStatusState.isRefreshing}<LoaderIcon class="size-4 animate-spin" />{text.holidayStatus.refreshing}{:else}<RefreshCwIcon class="size-4" />{text.holidayStatus.refresh}{/if}
			</Button>
		</Card.Footer>
	</Card.Root>
{/if}
