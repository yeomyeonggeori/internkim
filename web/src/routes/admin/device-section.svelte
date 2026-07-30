<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import {
		apiErrorMessage,
		applyBlueclawUpdate,
		fetchBlueclawUpdateJob,
		fetchBlueclawUpdateStatus,
		fetchDeviceHealth,
		fetchReleaseHistory
	} from './admin-api';
	import type { AdminJob, AdminPageText, AdminSession, BlueclawUpdateStatus, ReleaseHistoryEntry, ReleaseUpdateSummary } from './admin-types';

	type DeviceSectionProps = {
		adminBaseURL: string;
		adminSession: AdminSession | null;
		isDeviceReachable: boolean;
		fleetIdInput: string;
		text: AdminPageText;
		onFleetIDSaved: (fleetID: string) => Promise<void> | void;
	};

	let {
		adminBaseURL,
		adminSession,
		isDeviceReachable = $bindable(false),
		fleetIdInput = $bindable(''),
		text,
		onFleetIDSaved
	}: DeviceSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let isCheckingDevice = $state(false);
	let adminErrorMessage = $state('');
	let blueclawUpdateStatus = $state<BlueclawUpdateStatus | null>(null);
	let releaseHistoryEntries = $state<ReleaseHistoryEntry[]>([]);
	let blueclawUpdateJob = $state<AdminJob | null>(null);
	let blueclawUpdateMessage = $state('');
	let isLoadingBlueclawUpdate = $state(false);
	let isApplyingBlueclawUpdate = $state(false);
	let applyingReleaseID = $state('');

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		checkDevice();
	});

	function adminSessionStatusText() {
		if (!adminSession) return '';
		if (adminSession.isAdmin) return text.device.admin;
		if (adminSession.bootstrapStatus === 'identity_missing') return text.device.accessEmailMissing;
		if (adminSession.bootstrapStatus === 'failed') return text.device.claimFailed;
		if (adminSession.bootstrapStatus === 'rejected') return text.device.notAdmin;
		if (!adminSession.isClaimed) return text.device.firstAdminClaimPending;
		return text.device.notAdmin;
	}

	function shortRevision(revision: string | undefined) {
		const value = revision?.trim() ?? '';
		return value ? value.slice(0, 8) : text.deviceUpdate.notAvailable;
	}

	function releaseLabel(releaseID: string | undefined) {
		const value = releaseID?.trim() ?? '';
		if (!value) return text.deviceUpdate.notAvailable;
		const parsed = value.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})\d{2}Z-(.+)$/);
		if (!parsed) return value;
		const [, year, month, day, hour, minute, revision] = parsed;
		return `${year}-${month}-${day} ${hour}:${minute} (${revision.slice(0, 8)})`;
	}

	function releaseTimeFromID(releaseID: string | undefined) {
		const value = releaseID?.trim() ?? '';
		const parsed = value.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z-/);
		if (!parsed) return undefined;
		const [, year, month, day, hour, minute, second] = parsed;
		return Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), Number(second));
	}

	function releaseTimeFromDate(createdAt: string | undefined) {
		const value = createdAt?.trim() ?? '';
		if (!value) return undefined;
		const time = new Date(value).getTime();
		if (Number.isNaN(time)) return undefined;
		return time;
	}

	function currentReleaseTime() {
		const currentRelease = blueclawUpdateStatus?.current;
		return releaseTimeFromID(currentRelease?.releaseID) ?? releaseTimeFromDate(currentRelease?.createdAt);
	}

	function historyEntryReleaseTime(entry: ReleaseHistoryEntry) {
		return releaseTimeFromID(entry.releaseID) ?? releaseTimeFromDate(entry.createdAt);
	}

	function releaseComponents(summary: ReleaseUpdateSummary | undefined) {
		const components = summary?.components ?? {};
		return Object.entries(components).sort(([leftName], [rightName]) => leftName.localeCompare(rightName));
	}

	function releaseDate(createdAt: string | undefined) {
		const value = createdAt?.trim() ?? '';
		if (!value) return text.deviceUpdate.notAvailable;
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return date.toLocaleString('ko-KR', { dateStyle: 'medium', timeStyle: 'short' });
	}

	function isReleaseUpdateJobActive() {
		const status = blueclawUpdateJob?.status ?? blueclawUpdateStatus?.activeJob?.status ?? '';
		return status === 'pending' || status === 'running';
	}

	function isCurrentRelease(entry: ReleaseHistoryEntry) {
		const currentReleaseID = blueclawUpdateStatus?.current?.releaseID ?? '';
		return entry.isCurrent || (currentReleaseID !== '' && entry.releaseID === currentReleaseID);
	}

	function canApplyRelease(entry: ReleaseHistoryEntry) {
		if (!isDeviceReachable) return false;
		if (isApplyingBlueclawUpdate) return false;
		if (isReleaseUpdateJobActive()) return false;
		return !isCurrentRelease(entry);
	}

	function isDowngradeRelease(entry: ReleaseHistoryEntry) {
		const currentTime = currentReleaseTime();
		const targetTime = historyEntryReleaseTime(entry);
		if (currentTime === undefined || targetTime === undefined) return false;
		return targetTime < currentTime;
	}

	function releaseActionLabel(entry: ReleaseHistoryEntry) {
		return isDowngradeRelease(entry) ? text.deviceUpdate.downgrade : text.deviceUpdate.upgrade;
	}

	function releaseActionConfirmMessage(entry: ReleaseHistoryEntry) {
		const template = isDowngradeRelease(entry) ? text.deviceUpdate.downgradeConfirm : text.deviceUpdate.upgradeConfirm;
		return template.replace('{release}', releaseLabel(entry.releaseID));
	}

	function blueclawUpdateStateLabel(state: string | undefined) {
		const labels = text.deviceUpdate.states;
		if (state === 'current') return labels.current;
		if (state === 'update_available') return labels.updateAvailable;
		if (state === 'updating') return labels.updating;
		if (state === 'unknown') return labels.unknown;
		if (state === 'already_current') return labels.alreadyCurrent;
		if (state === 'completed') return labels.completed;
		if (state === 'failed') return labels.failed;
		if (state === 'running') return labels.running;
		if (state === 'checking') return labels.checking;
		if (state === 'downloading') return labels.downloading;
		if (state === 'verifying') return labels.verifying;
		if (state === 'installing') return labels.installing;
		if (state === 'restarting') return labels.restarting;
		return labels.idle;
	}

	async function saveFleetId() {
		fleetIdInput = fleetIdInput.trim().toLowerCase();
		await onFleetIDSaved(fleetIdInput);
		await checkDevice();
	}

	async function checkDevice() {
		if (!adminBaseURL) return;

		isCheckingDevice = true;
		adminErrorMessage = '';
		try {
			await fetchDeviceHealth(adminBaseURL, text.messages.deviceUnreachable);
			isDeviceReachable = true;
		} catch {
			isDeviceReachable = false;
			adminErrorMessage = text.messages.deviceUnreachable;
		} finally {
			isCheckingDevice = false;
		}
		if (!isDeviceReachable) {
			blueclawUpdateStatus = null;
			releaseHistoryEntries = [];
			blueclawUpdateJob = null;
			return;
		}
		await loadBlueclawUpdateStatus();
	}

	async function loadBlueclawUpdateStatus() {
		if (!adminBaseURL) return;

		isLoadingBlueclawUpdate = true;
		blueclawUpdateMessage = '';
		try {
			const [updateStatus, releaseHistory] = await Promise.all([
				fetchBlueclawUpdateStatus(adminBaseURL, text.deviceUpdate.loadError),
				fetchReleaseHistory(adminBaseURL, text.deviceUpdate.loadError)
			]);
			blueclawUpdateStatus = updateStatus;
			releaseHistoryEntries = releaseHistory.entries ?? [];
			blueclawUpdateJob = blueclawUpdateStatus.activeJob ?? blueclawUpdateJob;
		} catch (error) {
			blueclawUpdateMessage = apiErrorMessage(error, text.deviceUpdate.loadError);
		} finally {
			isLoadingBlueclawUpdate = false;
		}
	}

	async function applyUpdate(entry: ReleaseHistoryEntry) {
		if (!adminBaseURL) return;
		if (!confirm(releaseActionConfirmMessage(entry))) return;

		isApplyingBlueclawUpdate = true;
		const releaseID = entry.releaseID;
		applyingReleaseID = releaseID;
		blueclawUpdateMessage = '';
		try {
			blueclawUpdateJob = await applyBlueclawUpdate(adminBaseURL, text.deviceUpdate.applyError, releaseID);
			await pollBlueclawUpdateJob(blueclawUpdateJob.jobID);
		} catch (error) {
			blueclawUpdateMessage = apiErrorMessage(error, text.deviceUpdate.applyError);
		} finally {
			isApplyingBlueclawUpdate = false;
			applyingReleaseID = '';
		}
	}

	async function pollBlueclawUpdateJob(jobID: string) {
		for (let attempt = 0; attempt < 120; attempt += 1) {
			try {
				blueclawUpdateJob = await fetchBlueclawUpdateJob(adminBaseURL, jobID, text.deviceUpdate.loadError);
				if (blueclawUpdateJob.status === 'completed' || blueclawUpdateJob.status === 'failed' || blueclawUpdateJob.status === 'already_current') {
					await loadBlueclawUpdateStatus();
					return;
				}
			} catch {
				return;
			}
			await new Promise((resolve) => setTimeout(resolve, 1500));
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.device.title}</Card.Title>
		<Card.Description>{text.device.description}</Card.Description>
		<Card.Action>
			<Badge variant={isDeviceReachable ? 'secondary' : 'outline'}>
				{isDeviceReachable ? text.device.online : text.device.unreachable}
			</Badge>
		</Card.Action>
	</Card.Header>
	<Card.Content>
		<Field.Group>
			<form
				onsubmit={(event) => {
					event.preventDefault();
					saveFleetId();
				}}
			>
				<Field.Field>
					<Field.Label for="device-fleet-id">{text.device.fleetIDPlaceholder}</Field.Label>
					<div class="flex flex-wrap items-center gap-2">
						<Input id="device-fleet-id" class="min-w-64 flex-1" bind:value={fleetIdInput} autocomplete="off" />
						<Button type="submit" variant="outline">
							{#if isCheckingDevice}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<RefreshCwIcon />
							{/if}
							{text.device.check}
						</Button>
					</div>
					{#if adminSession?.email}
						<Field.Description>
							Cloudflare Access: <span class="text-foreground font-medium">{adminSession.email}</span>
							{#if adminSession.isAdmin}
								· {text.device.admin}
							{:else}
								· {adminSession.bootstrapStatus === 'failed' ? text.device.claimFailed : adminSessionStatusText()}
							{/if}
						</Field.Description>
					{:else if adminSession}
						<Field.Description>Cloudflare Access: {adminSessionStatusText()}</Field.Description>
					{/if}
				</Field.Field>
			</form>
			{#if adminErrorMessage || adminSession?.bootstrapError}
				<Field.Error>{adminErrorMessage || adminSession?.bootstrapError}</Field.Error>
			{/if}
		</Field.Group>
	</Card.Content>
</Card.Root>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.deviceUpdate.title}</Card.Title>
		<Card.Description>{text.deviceUpdate.description}</Card.Description>
		<Card.Action class="flex items-center gap-2">
			<Badge variant={blueclawUpdateStatus?.state === 'current' ? 'secondary' : 'outline'}>
				{blueclawUpdateStateLabel(blueclawUpdateJob?.phase || blueclawUpdateJob?.status || blueclawUpdateStatus?.state)}
			</Badge>
			<Button variant="outline" size="sm" disabled={!isDeviceReachable || isLoadingBlueclawUpdate} onclick={loadBlueclawUpdateStatus}>
				{#if isLoadingBlueclawUpdate}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<RefreshCwIcon />
				{/if}
				{text.deviceUpdate.refresh}
			</Button>
		</Card.Action>
	</Card.Header>
	<Card.Content class="grid gap-5">
		<dl class="grid gap-4 sm:grid-cols-3">
			<div class="grid gap-1">
				<dt class="text-muted-foreground text-xs">{text.deviceUpdate.currentRevision}</dt>
				<dd class="font-mono text-sm">{releaseLabel(blueclawUpdateStatus?.current?.releaseID)}</dd>
			</div>
			<div class="grid gap-1">
				<dt class="text-muted-foreground text-xs">{text.deviceUpdate.latestRevision}</dt>
				<dd class="font-mono text-sm">{releaseLabel(blueclawUpdateStatus?.latest?.releaseID)}</dd>
			</div>
			<div class="grid gap-1">
				<dt class="text-muted-foreground text-xs">{text.deviceUpdate.jobPhase}</dt>
				<dd class="text-sm">{blueclawUpdateStateLabel(blueclawUpdateJob?.phase || blueclawUpdateJob?.status || blueclawUpdateStatus?.state)}</dd>
			</div>
		</dl>

		{#if releaseComponents(blueclawUpdateStatus?.latest).length > 0}
			<Field.Field>
				<Field.Label>{text.deviceUpdate.components}</Field.Label>
				<div class="grid gap-2 sm:grid-cols-2">
					{#each releaseComponents(blueclawUpdateStatus?.latest) as [componentName, component]}
						<Item.Root variant="outline" size="xs">
							<Item.Content>
								<Item.Title>{componentName}</Item.Title>
							</Item.Content>
							<Item.Actions class="text-muted-foreground font-mono text-xs">{shortRevision(component.revision)}</Item.Actions>
						</Item.Root>
					{/each}
				</div>
			</Field.Field>
		{/if}

		<Field.Field>
			<Field.Label>{text.deviceUpdate.recentReleases}</Field.Label>
			{#if releaseHistoryEntries.length > 0}
				<Item.Group class="gap-2">
					{#each releaseHistoryEntries as entry}
						<Item.Root variant="outline">
							<Item.Content>
								<Item.Title class="font-mono">
									{releaseLabel(entry.releaseID)}
									{#if isCurrentRelease(entry)}
										<Badge variant="secondary" class="font-sans">{text.deviceUpdate.currentBadge}</Badge>
									{/if}
								</Item.Title>
								<Item.Description>{releaseDate(entry.createdAt)}</Item.Description>
							</Item.Content>
							<Item.Actions>
								<Button size="sm" variant="outline" disabled={!canApplyRelease(entry)} onclick={() => applyUpdate(entry)}>
									{#if isApplyingBlueclawUpdate && applyingReleaseID === entry.releaseID}
										<LoaderIcon class="size-4 animate-spin" />
									{:else}
										<UploadIcon />
									{/if}
									{releaseActionLabel(entry)}
								</Button>
							</Item.Actions>
						</Item.Root>
					{/each}
				</Item.Group>
			{:else}
				<p class="text-muted-foreground rounded-lg border border-dashed px-3 py-6 text-center text-sm">{text.deviceUpdate.noReleases}</p>
			{/if}
			<Field.Description>{text.deviceUpdate.notice}</Field.Description>
		</Field.Field>

		{#if blueclawUpdateMessage || blueclawUpdateJob?.error}
			<Field.Error>{blueclawUpdateMessage || blueclawUpdateJob?.error}</Field.Error>
		{/if}
	</Card.Content>
</Card.Root>
