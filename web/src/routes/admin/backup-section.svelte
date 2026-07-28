<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
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

{#snippet jobStatus(job: AdminJob, fallbackFleetID: string)}
	<Item.Root variant="muted" class="items-start">
		<Item.Content>
			<Item.Title>{job.status} · {job.phase}</Item.Title>
			{#if job.manifest}
				<Item.Description>
					{job.manifest.fleetID || fallbackFleetID} · {job.manifest.components?.join(', ') || text.backup.manifestReady}
				</Item.Description>
			{/if}
			{#if job.error}
				<Item.Description class="text-destructive">{job.error}</Item.Description>
			{/if}
		</Item.Content>
	</Item.Root>
{/snippet}

<div class="grid items-start gap-5 lg:grid-cols-2">
	<Card.Root>
		<Card.Header class="border-b pb-4">
			<Card.Title>{text.backup.title}</Card.Title>
			<Card.Description>{text.backup.description}</Card.Description>
		</Card.Header>
		<Card.Content>
			<Field.Group>
				<Field.Field>
					<Field.Label for="backup-passphrase">{text.backup.passphrasePlaceholder}</Field.Label>
					<Input id="backup-passphrase" bind:value={backupPassphrase} type="password" autocomplete="new-password" />
				</Field.Field>
				{#if backupJob}
					{@render jobStatus(backupJob, fleetID)}
				{/if}
			</Field.Group>
		</Card.Content>
		<Card.Footer class="gap-2">
			<Button disabled={!isDeviceReachable || isCreatingBackup || !backupPassphrase.trim()} onclick={createEncryptedBackup}>
				{#if isCreatingBackup}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<DownloadIcon />
				{/if}
				{text.backup.create}
			</Button>
			{#if backupDownloadURL()}
				<Button href={backupDownloadURL()} variant="outline">
					<DownloadIcon />
					{text.backup.download}
				</Button>
			{/if}
		</Card.Footer>
	</Card.Root>

	<Card.Root>
		<Card.Header class="border-b pb-4">
			<Card.Title>{text.backup.restoreTitle}</Card.Title>
			<Card.Description>{text.backup.restoreDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<Field.Group>
				<Field.Field>
					<Field.Label for="restore-bundle">{text.backup.bundleLabel}</Field.Label>
					<Input id="restore-bundle" type="file" accept=".ikbak,application/octet-stream" onchange={handleRestoreFile} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="restore-passphrase">{text.backup.passphrasePlaceholder}</Field.Label>
					<Input id="restore-passphrase" bind:value={restorePassphrase} type="password" autocomplete="new-password" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="restore-confirm">{text.backup.restoreConfirmLabel}</Field.Label>
					<Input id="restore-confirm" bind:value={restoreConfirm} placeholder={text.backup.restoreConfirmPlaceholder} autocomplete="off" />
				</Field.Field>
				{#if restoreJob}
					{@render jobStatus(restoreJob, text.backup.backupFallback)}
				{/if}
			</Field.Group>
		</Card.Content>
		<Card.Footer>
			<Button
				disabled={!isDeviceReachable || isRestoring || !restoreBundle || !restorePassphrase.trim() || restoreConfirm.trim() !== 'RESTORE'}
				onclick={restoreBackup}
			>
				{#if isRestoring}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<UploadIcon />
				{/if}
				{text.backup.restore}
			</Button>
		</Card.Footer>
	</Card.Root>
</div>

{#if adminErrorMessage}
	<Field.Error>{adminErrorMessage}</Field.Error>
{/if}
