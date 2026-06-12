<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import {
		apiErrorMessage,
		completeRestoreUpload,
		createBackup,
		createRestoreUpload,
		fetchBackupJob,
		fetchRestoreJob,
		uploadRestoreChunk
	} from './admin-api';
	import type { AdminJob, AdminPageText } from './admin-types';

	type BackupSectionProps = {
		adminBaseURL: string;
		fleetID: string;
		isDeviceReachable: boolean;
		isDeviceHost: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, fleetID, isDeviceReachable, isDeviceHost, text }: BackupSectionProps = $props();

	let backupPassphrase = $state('');
	let restorePassphrase = $state('');
	let restoreConfirm = $state('');
	let restoreBundle = $state<File | null>(null);
	let adminErrorMessage = $state('');
	let backupJob = $state<AdminJob | null>(null);
	let restoreJob = $state<AdminJob | null>(null);
	let isCreatingBackup = $state(false);
	let isRestoring = $state(false);

	function backupDownloadURL() {
		if (!backupJob?.downloadURL || !fleetID) return '';
		if (isDeviceHost) return backupJob.downloadURL;
		return `https://${fleetID}.example.test${backupJob.downloadURL}`;
	}

	async function createEncryptedBackup() {
		if (!adminBaseURL || !backupPassphrase.trim()) return;

		isCreatingBackup = true;
		adminErrorMessage = '';
		try {
			backupJob = await createBackup(adminBaseURL, backupPassphrase, text.messages.backupStartError);
			await pollJob('backup', backupJob.jobID);
		} catch {
			adminErrorMessage = text.messages.backupStartError;
		} finally {
			isCreatingBackup = false;
		}
	}

	async function restoreBackup() {
		if (!adminBaseURL || !restoreBundle || !restorePassphrase.trim() || restoreConfirm.trim() !== 'RESTORE') return;

		isRestoring = true;
		adminErrorMessage = '';
		try {
			const upload = await createRestoreUpload(adminBaseURL, restoreBundle, text.messages.restoreStartError);
			const chunkCount = Math.ceil(restoreBundle.size / upload.chunkSize);
			for (let chunkIndex = 0; chunkIndex < chunkCount; chunkIndex += 1) {
				const start = chunkIndex * upload.chunkSize;
				const end = Math.min(start + upload.chunkSize, restoreBundle.size);
				adminErrorMessage = `${text.messages.restoreUploadProgress} ${chunkIndex + 1}/${chunkCount}`;
				await uploadRestoreChunk(adminBaseURL, upload.uploadID, chunkIndex, restoreBundle.slice(start, end), text.messages.restoreUploadError);
			}
			adminErrorMessage = '';
			restoreJob = await completeRestoreUpload(
				adminBaseURL,
				upload.uploadID,
				{ passphrase: restorePassphrase, confirm: restoreConfirm.trim(), chunks: chunkCount },
				text.messages.restoreStartError
			);
			await pollJob('restore', restoreJob.jobID);
		} catch (error) {
			adminErrorMessage = apiErrorMessage(error, text.messages.restoreStartError);
		} finally {
			isRestoring = false;
		}
	}

	async function pollJob(jobType: 'backup' | 'restore', jobID: string) {
		for (let attempt = 0; attempt < 120; attempt += 1) {
			try {
				const job = jobType === 'backup'
					? await fetchBackupJob(adminBaseURL, jobID, text.messages.backupStartError)
					: await fetchRestoreJob(adminBaseURL, jobID, text.messages.restoreStartError);
				if (jobType === 'backup') backupJob = job;
				if (jobType === 'restore') restoreJob = job;
				if (job.status === 'completed' || job.status === 'failed') return;
			} catch {
				return;
			}
			await new Promise((resolve) => setTimeout(resolve, 1500));
		}
	}

	function handleRestoreFile(event: Event) {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		const input = event.currentTarget;
		restoreBundle = input.files?.[0] ?? null;
	}
</script>

{#if adminErrorMessage}
	<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{adminErrorMessage}</p>
{/if}

<div class="grid gap-4 md:grid-cols-2">
	<div class="rounded-lg border p-4">
		<div class="mb-4">
			<h3 class="text-sm font-semibold">{text.backup.title}</h3>
			<p class="text-muted-foreground mt-1 text-sm">{text.backup.description}</p>
		</div>
		<div class="grid gap-3">
			<Input bind:value={backupPassphrase} type="password" placeholder={text.backup.passphrasePlaceholder} autocomplete="new-password" />
			<Button disabled={!isDeviceReachable || isCreatingBackup || !backupPassphrase.trim()} onclick={createEncryptedBackup} class="gap-2">
				{#if isCreatingBackup}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<DownloadIcon class="size-4" />
				{/if}
				{text.backup.create}
			</Button>
			{#if backupJob}
				<div class="rounded-md bg-muted/40 p-3 text-sm">
					<p class="font-medium">{backupJob.status} · {backupJob.phase}</p>
					{#if backupJob.error}
						<p class="mt-1 text-destructive">{backupJob.error}</p>
					{/if}
					{#if backupJob.manifest}
						<p class="text-muted-foreground mt-2">
							{backupJob.manifest.fleetID || fleetID} · {backupJob.manifest.components?.join(', ') || text.backup.manifestReady}
						</p>
					{/if}
				</div>
			{/if}
			{#if backupDownloadURL()}
				<Button href={backupDownloadURL()} variant="outline" class="gap-2">
					<DownloadIcon class="size-4" />
					{text.backup.download}
				</Button>
			{/if}
		</div>
	</div>

	<div class="rounded-lg border p-4">
		<div class="mb-4">
			<h3 class="text-sm font-semibold">{text.backup.restoreTitle}</h3>
			<p class="text-muted-foreground mt-1 text-sm">{text.backup.restoreDescription}</p>
		</div>
		<div class="grid gap-3">
			<Input type="file" accept=".ikbak,application/octet-stream" onchange={handleRestoreFile} />
			<Input bind:value={restorePassphrase} type="password" placeholder={text.backup.passphrasePlaceholder} autocomplete="new-password" />
			<Input bind:value={restoreConfirm} placeholder={text.backup.restoreConfirmPlaceholder} autocomplete="off" />
			<Button
				disabled={!isDeviceReachable || isRestoring || !restoreBundle || !restorePassphrase.trim() || restoreConfirm.trim() !== 'RESTORE'}
				onclick={restoreBackup}
				class="gap-2"
			>
				{#if isRestoring}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<UploadIcon class="size-4" />
				{/if}
				{text.backup.restore}
			</Button>
			{#if restoreJob}
				<div class="rounded-md bg-muted/40 p-3 text-sm">
					<p class="font-medium">{restoreJob.status} · {restoreJob.phase}</p>
					{#if restoreJob.error}
						<p class="mt-1 text-destructive">{restoreJob.error}</p>
					{/if}
					{#if restoreJob.manifest}
						<p class="text-muted-foreground mt-2">
							{restoreJob.manifest.fleetID || text.backup.backupFallback} · {restoreJob.manifest.components?.join(', ') || text.backup.manifestReady}
						</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</div>
