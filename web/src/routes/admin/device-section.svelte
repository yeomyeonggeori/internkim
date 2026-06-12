<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import {
		apiErrorMessage,
		applyBlueclawUpdate,
		fetchBlueclawUpdateJob,
		fetchBlueclawUpdateStatus,
		fetchDeviceHealth
	} from './admin-api';
	import type { AdminJob, AdminPageText, AdminSession, BlueclawUpdateStatus, ReleaseUpdateSummary } from './admin-types';

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
	let blueclawUpdateJob = $state<AdminJob | null>(null);
	let blueclawUpdateMessage = $state('');
	let isLoadingBlueclawUpdate = $state(false);
	let isApplyingBlueclawUpdate = $state(false);

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

	function releaseComponents(summary: ReleaseUpdateSummary | undefined) {
		const components = summary?.components ?? {};
		return Object.entries(components).sort(([leftName], [rightName]) => leftName.localeCompare(rightName));
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
			blueclawUpdateStatus = await fetchBlueclawUpdateStatus(adminBaseURL, text.deviceUpdate.loadError);
			blueclawUpdateJob = blueclawUpdateStatus.activeJob ?? blueclawUpdateJob;
		} catch (error) {
			blueclawUpdateMessage = apiErrorMessage(error, text.deviceUpdate.loadError);
		} finally {
			isLoadingBlueclawUpdate = false;
		}
	}

	async function applyUpdate() {
		if (!adminBaseURL) return;

		isApplyingBlueclawUpdate = true;
		blueclawUpdateMessage = '';
		try {
			blueclawUpdateJob = await applyBlueclawUpdate(adminBaseURL, text.deviceUpdate.applyError);
			await pollBlueclawUpdateJob(blueclawUpdateJob.jobID);
		} catch (error) {
			blueclawUpdateMessage = apiErrorMessage(error, text.deviceUpdate.applyError);
		} finally {
			isApplyingBlueclawUpdate = false;
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

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		<h2 class="text-base font-semibold">{text.device.title}</h2>
		<p class="text-muted-foreground text-sm">{text.device.description}</p>
		{#if adminSession?.email}
			<p class="text-muted-foreground mt-1 text-xs">
				Cloudflare Access: <span class="font-medium text-foreground">{adminSession.email}</span>
				{#if adminSession.isAdmin}
					<span class="ml-1 text-emerald-700">{text.device.admin}</span>
				{:else if adminSession.bootstrapStatus === 'failed'}
					<span class="ml-1 text-destructive">{text.device.claimFailed}</span>
				{:else}
					<span class="ml-1 text-amber-700">{adminSessionStatusText()}</span>
				{/if}
			</p>
			{#if adminSession.bootstrapError}
				<p class="mt-1 text-xs text-destructive">{adminSession.bootstrapError}</p>
			{/if}
		{:else if adminSession}
			<p class="text-muted-foreground mt-1 text-xs">
				Cloudflare Access: <span class="font-medium text-destructive">{adminSessionStatusText()}</span>
			</p>
		{/if}
	</div>
	<Badge variant={isDeviceReachable ? 'secondary' : 'outline'}>
		{isDeviceReachable ? text.device.online : text.device.unreachable}
	</Badge>
</div>

<form
	class="grid gap-2 sm:grid-cols-[1fr_auto]"
	onsubmit={(event) => {
		event.preventDefault();
		saveFleetId();
	}}
>
	<Input bind:value={fleetIdInput} placeholder={text.device.fleetIDPlaceholder} autocomplete="off" />
	<Button type="submit" variant="outline" class="gap-2">
		{#if isCheckingDevice}
			<LoaderIcon class="size-4 animate-spin" />
		{:else}
			<RefreshCwIcon class="size-4" />
		{/if}
		{text.device.check}
	</Button>
</form>

{#if adminErrorMessage}
	<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{adminErrorMessage}</p>
{/if}

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="text-sm font-semibold">{text.deviceUpdate.title}</h3>
			<p class="text-muted-foreground mt-1 text-sm">{text.deviceUpdate.description}</p>
		</div>
		<Badge variant={blueclawUpdateStatus?.state === 'current' ? 'secondary' : 'outline'}>
			{blueclawUpdateStateLabel(blueclawUpdateJob?.phase || blueclawUpdateJob?.status || blueclawUpdateStatus?.state)}
		</Badge>
	</div>
	<div class="grid gap-3 md:grid-cols-3">
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-muted-foreground text-xs">{text.deviceUpdate.currentRevision}</p>
			<p class="mt-1 font-mono text-sm">{releaseLabel(blueclawUpdateStatus?.current?.releaseID)}</p>
		</div>
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-muted-foreground text-xs">{text.deviceUpdate.latestRevision}</p>
			<p class="mt-1 font-mono text-sm">{releaseLabel(blueclawUpdateStatus?.latest?.releaseID)}</p>
		</div>
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-muted-foreground text-xs">{text.deviceUpdate.jobPhase}</p>
			<p class="mt-1 text-sm">{blueclawUpdateStateLabel(blueclawUpdateJob?.phase || blueclawUpdateJob?.status || blueclawUpdateStatus?.state)}</p>
		</div>
	</div>
	{#if releaseComponents(blueclawUpdateStatus?.latest).length > 0}
		<div class="mt-3 rounded-md bg-muted/20 p-3">
			<p class="text-muted-foreground mb-2 text-xs">{text.deviceUpdate.components}</p>
			<div class="grid gap-2 sm:grid-cols-2">
				{#each releaseComponents(blueclawUpdateStatus?.latest) as [componentName, component]}
					<div class="flex items-center justify-between gap-3 rounded border bg-background px-2 py-1.5 text-xs">
						<span class="font-medium">{componentName}</span>
						<span class="font-mono text-muted-foreground">{shortRevision(component.revision)}</span>
					</div>
				{/each}
			</div>
		</div>
	{/if}
	<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
		<p class="text-muted-foreground text-xs">{text.deviceUpdate.notice}</p>
		<div class="flex gap-2">
			<Button variant="outline" size="sm" class="gap-2" disabled={!isDeviceReachable || isLoadingBlueclawUpdate} onclick={loadBlueclawUpdateStatus}>
				{#if isLoadingBlueclawUpdate}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<RefreshCwIcon class="size-4" />
				{/if}
				{text.deviceUpdate.refresh}
			</Button>
			<Button
				size="sm"
				class="gap-2"
				disabled={!isDeviceReachable || isApplyingBlueclawUpdate || !blueclawUpdateStatus?.updateAllowed || blueclawUpdateStatus?.state === 'current'}
				onclick={applyUpdate}
			>
				{#if isApplyingBlueclawUpdate}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<UploadIcon class="size-4" />
				{/if}
				{text.deviceUpdate.apply}
			</Button>
		</div>
	</div>
	{#if blueclawUpdateMessage || blueclawUpdateJob?.error}
		<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
			{blueclawUpdateMessage || blueclawUpdateJob?.error}
		</p>
	{/if}
</div>
