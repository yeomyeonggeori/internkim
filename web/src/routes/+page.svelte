<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import QrCode from 'svelte-qrcode';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';

	type UsersResponse = {
		users?: string[];
	};

	type BackupManifest = {
		deviceID?: string;
		createdAt?: string;
		components?: string[];
		mattermostDump?: boolean;
	};

	type AdminJob = {
		jobID: string;
		type: string;
		status: string;
		phase: string;
		error?: string;
		downloadURL?: string;
		manifest?: BackupManifest;
		logs?: string[];
	};

	type RestoreUploadResponse = {
		uploadID: string;
		chunkSize: number;
	};

	const logoSrc = '/logo.svg';
	const storedDeviceIdKey = 'internkim_device_id';

	let deviceIdInput = $state('');
	let userEmails = $state<string[]>([]);
	let newEmail = $state('');
	let isLoadingUsers = $state(false);
	let isSavingUser = $state(false);
	let errorMessage = $state('');
	let isCheckingDevice = $state(false);
	let isDeviceReachable = $state(false);
	let backupPassphrase = $state('');
	let restorePassphrase = $state('');
	let restoreConfirm = $state('');
	let restoreBundle = $state<File | null>(null);
	let adminErrorMessage = $state('');
	let backupJob = $state<AdminJob | null>(null);
	let restoreJob = $state<AdminJob | null>(null);
	let isCreatingBackup = $state(false);
	let isRestoring = $state(false);

	const deviceId = () => {
		const explicitId = deviceIdInput.trim().toLowerCase();
		if (explicitId) return explicitId;
		if (!browser) return '';
		return deviceIdFromHost();
	};

	const pagesApi = () => {
		const id = deviceId();
		return id ? `https://api.intern.kim/api` : '/api';
	};

	const mattermostURL = () => {
		const id = deviceId();
		return id ? `https://${id}.intern.kim` : '';
	};

	const isDeviceContext = () => deviceId() !== '';
	const adminBaseURL = () => {
		const id = deviceId();
		return id ? `https://${id}.intern.kim/_internkim/admin` : '';
	};
	const backupDownloadURL = () => {
		if (!backupJob?.downloadURL || !deviceId()) return '';
		return `https://${deviceId()}.intern.kim${backupJob.downloadURL}`;
	};

	onMount(() => {
		const queryDeviceId = new URLSearchParams(location.search).get('device_id')?.trim().toLowerCase() ?? '';
		deviceIdInput = queryDeviceId || deviceIdFromHost() || localStorage.getItem(storedDeviceIdKey) || '';
		if (deviceIdInput) localStorage.setItem(storedDeviceIdKey, deviceIdInput);
		loadUsers();
		checkDevice();
	});

	function deviceIdFromHost() {
		if (!browser) return '';
		const host = location.hostname;
		if (host === 'localhost' || host === '127.0.0.1' || /^\d+\.\d+\.\d+\.\d+$/.test(host)) return '';
		const suffix = '.intern.kim';
		if (!host.endsWith(suffix)) return '';
		const id = host.slice(0, -suffix.length);
		if (!id || id === 'api' || id.includes('.')) return '';
		return id;
	}

	function saveDeviceId() {
		deviceIdInput = deviceIdInput.trim().toLowerCase();
		if (deviceIdInput) localStorage.setItem(storedDeviceIdKey, deviceIdInput);
		loadUsers();
		checkDevice();
	}

	async function loadUsers() {
		const id = deviceId();
		if (!id) return;

		isLoadingUsers = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users?device_id=${id}`);
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 초대 목록을 볼 수 있습니다.' : '초대 목록을 불러오지 못했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
		} catch {
			errorMessage = '초대 목록을 불러오지 못했습니다.';
		} finally {
			isLoadingUsers = false;
		}
	}

	async function addEmail() {
		const email = newEmail.trim().toLowerCase();
		if (!email || !deviceId()) return;

		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ device_id: deviceId(), email })
			});
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 사용자를 초대할 수 있습니다.' : '사용자 초대에 실패했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
			newEmail = '';
		} catch {
			errorMessage = '사용자 초대에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}

	async function removeEmail(email: string) {
		if (!deviceId()) return;

		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${pagesApi()}/users/${encodeURIComponent(email)}?device_id=${deviceId()}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				errorMessage = response.status === 403 ? '관리자만 사용자를 제거할 수 있습니다.' : '사용자 제거에 실패했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			userEmails = data.users ?? [];
		} catch {
			errorMessage = '사용자 제거에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}

	async function checkDevice() {
		if (!adminBaseURL()) return;

		isCheckingDevice = true;
		adminErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/health`, { credentials: 'include' });
			isDeviceReachable = response.ok;
			if (!response.ok) adminErrorMessage = '기기에 연결할 수 없습니다.';
		} catch {
			isDeviceReachable = false;
			adminErrorMessage = '기기에 연결할 수 없습니다.';
		} finally {
			isCheckingDevice = false;
		}
	}

	async function createBackup() {
		if (!adminBaseURL() || !backupPassphrase.trim()) return;

		isCreatingBackup = true;
		adminErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/backups`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ passphrase: backupPassphrase })
			});
			if (!response.ok) {
				adminErrorMessage = '백업을 시작하지 못했습니다.';
				return;
			}
			backupJob = (await response.json()) as AdminJob;
			await pollJob('backup', backupJob.jobID);
		} catch {
			adminErrorMessage = '백업을 시작하지 못했습니다.';
		} finally {
			isCreatingBackup = false;
		}
	}

	async function restoreBackup() {
		if (!adminBaseURL() || !restoreBundle || !restorePassphrase.trim() || restoreConfirm.trim() !== 'RESTORE') return;

		isRestoring = true;
		adminErrorMessage = '';
		try {
			const uploadResponse = await fetch(`${adminBaseURL()}/restore/uploads`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ filename: restoreBundle.name, size: restoreBundle.size })
			});
			if (!uploadResponse.ok) {
				adminErrorMessage = '복구를 시작하지 못했습니다.';
				return;
			}
			const upload = (await uploadResponse.json()) as RestoreUploadResponse;
			const chunkCount = Math.ceil(restoreBundle.size / upload.chunkSize);
			for (let chunkIndex = 0; chunkIndex < chunkCount; chunkIndex += 1) {
				const start = chunkIndex * upload.chunkSize;
				const end = Math.min(start + upload.chunkSize, restoreBundle.size);
				adminErrorMessage = `복구 파일 업로드 중... ${chunkIndex + 1}/${chunkCount}`;
				const chunkResponse = await fetch(`${adminBaseURL()}/restore/uploads/${upload.uploadID}/chunks/${chunkIndex}`, {
					method: 'PUT',
					credentials: 'include',
					body: restoreBundle.slice(start, end)
				});
				if (!chunkResponse.ok) {
					adminErrorMessage = '복구 파일 업로드에 실패했습니다.';
					return;
				}
			}
			adminErrorMessage = '';
			const completeResponse = await fetch(`${adminBaseURL()}/restore/uploads/${upload.uploadID}/complete`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ passphrase: restorePassphrase, confirm: restoreConfirm.trim(), chunks: chunkCount })
			});
			if (!completeResponse.ok) {
				adminErrorMessage = '복구를 시작하지 못했습니다.';
				return;
			}
			restoreJob = (await completeResponse.json()) as AdminJob;
			await pollJob('restore', restoreJob.jobID);
		} catch {
			adminErrorMessage = '복구를 시작하지 못했습니다.';
		} finally {
			isRestoring = false;
		}
	}

	async function pollJob(jobType: 'backup' | 'restore', jobID: string) {
		const pathName = jobType === 'backup' ? 'backups' : 'restore';
		for (let attempt = 0; attempt < 120; attempt += 1) {
			const response = await fetch(`${adminBaseURL()}/${pathName}/${jobID}/status`, { credentials: 'include' });
			if (response.ok) {
				const job = (await response.json()) as AdminJob;
				if (jobType === 'backup') backupJob = job;
				if (jobType === 'restore') restoreJob = job;
				if (job.status === 'completed' || job.status === 'failed') return;
			}
			await new Promise((resolve) => setTimeout(resolve, 1500));
		}
	}

	function handleRestoreFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		restoreBundle = input.files?.[0] ?? null;
	}
</script>

<svelte:head>
	<title>intern kim</title>
</svelte:head>

<main class="bg-background text-foreground min-h-svh">
	<div class="mx-auto flex min-h-svh w-full max-w-4xl flex-col px-5 py-6">
		<header class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<img src={logoSrc} alt="intern kim" class="size-9" />
				<div>
					<h1 class="text-lg font-semibold leading-tight">intern kim</h1>
					<p class="text-muted-foreground text-sm">Mattermost와 Slack에서 사용하는 사내 AI 하드웨어</p>
				</div>
			</div>
			<Badge variant="secondary" class="gap-1.5">
				<ShieldCheckIcon class="size-3.5" />
				Access protected
			</Badge>
		</header>

		<section class="grid gap-4 py-8 sm:grid-cols-[190px_1fr]">
			<div class="flex items-center justify-center rounded-lg border bg-muted/30 p-4">
				{#if mattermostURL()}
					<QrCode value={mattermostURL()} size="150" />
				{:else}
					<MessageSquareIcon class="text-muted-foreground size-16" strokeWidth={1.5} />
				{/if}
			</div>
			<div class="flex min-w-0 flex-col justify-center gap-4">
				<div>
					<h2 class="text-2xl font-semibold">대화는 Mattermost에서 시작하세요.</h2>
					<p class="text-muted-foreground mt-2 text-sm leading-6">
						초대받은 팀원은 Mattermost와 Slack에서 Intern Kim에게 바로 일을 맡길 수 있습니다.
					</p>
				</div>
				{#if mattermostURL()}
					<div class="flex flex-wrap items-center gap-2">
						<Button href={mattermostURL()} class="gap-2">
							<ExternalLinkIcon class="size-4" />
							Open Mattermost
						</Button>
						<CopyButton text={mattermostURL()} variant="outline" />
					</div>
				{:else}
					<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
						기기 등록이 끝나면 전용 Mattermost 주소와 초대 관리가 표시됩니다.
					</p>
				{/if}
			</div>
		</section>

		<Separator />

		<section class="grid gap-5 py-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">Device Admin</h2>
					<p class="text-muted-foreground text-sm">원격 기기의 상태와 암호화 백업을 관리합니다.</p>
				</div>
				<Badge variant={isDeviceReachable ? 'secondary' : 'outline'}>
					{isDeviceReachable ? 'online' : 'unreachable'}
				</Badge>
			</div>

			<form
				class="grid gap-2 sm:grid-cols-[1fr_auto]"
				onsubmit={(event) => {
					event.preventDefault();
					saveDeviceId();
				}}
			>
				<Input bind:value={deviceIdInput} placeholder="device id" autocomplete="off" />
				<Button type="submit" variant="outline" class="gap-2">
					{#if isCheckingDevice}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<RefreshCwIcon class="size-4" />
					{/if}
					Check
				</Button>
			</form>

			{#if adminErrorMessage}
				<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{adminErrorMessage}</p>
			{/if}

			<div class="grid gap-4 md:grid-cols-2">
				<div class="rounded-lg border p-4">
					<div class="mb-4">
						<h3 class="text-sm font-semibold">Encrypted Backup</h3>
						<p class="text-muted-foreground mt-1 text-sm">passphrase는 브라우저에서 원격 기기로만 전송됩니다.</p>
					</div>
					<div class="grid gap-3">
						<Input bind:value={backupPassphrase} type="password" placeholder="backup passphrase" autocomplete="new-password" />
						<Button disabled={!isDeviceReachable || isCreatingBackup || !backupPassphrase.trim()} onclick={createBackup} class="gap-2">
							{#if isCreatingBackup}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<DownloadIcon class="size-4" />
							{/if}
							Create encrypted backup
						</Button>
						{#if backupJob}
							<div class="rounded-md bg-muted/40 p-3 text-sm">
								<p class="font-medium">{backupJob.status} · {backupJob.phase}</p>
								{#if backupJob.error}
									<p class="mt-1 text-destructive">{backupJob.error}</p>
								{/if}
								{#if backupJob.manifest}
									<p class="text-muted-foreground mt-2">
										{backupJob.manifest.deviceID || deviceId()} · {backupJob.manifest.components?.join(', ') || 'manifest ready'}
									</p>
								{/if}
							</div>
						{/if}
						{#if backupDownloadURL()}
							<Button href={backupDownloadURL()} variant="outline" class="gap-2">
								<DownloadIcon class="size-4" />
								Download backup
							</Button>
						{/if}
					</div>
				</div>

				<div class="rounded-lg border p-4">
					<div class="mb-4">
						<h3 class="text-sm font-semibold">Restore</h3>
						<p class="text-muted-foreground mt-1 text-sm">복구하려는 대상 기기를 확인한 뒤 RESTORE를 입력하세요.</p>
					</div>
					<div class="grid gap-3">
						<Input type="file" accept=".ikbak,application/octet-stream" onchange={handleRestoreFile} />
						<Input bind:value={restorePassphrase} type="password" placeholder="backup passphrase" autocomplete="new-password" />
						<Input bind:value={restoreConfirm} placeholder="type RESTORE" autocomplete="off" />
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
							Restore device
						</Button>
						{#if restoreJob}
							<div class="rounded-md bg-muted/40 p-3 text-sm">
								<p class="font-medium">{restoreJob.status} · {restoreJob.phase}</p>
								{#if restoreJob.error}
									<p class="mt-1 text-destructive">{restoreJob.error}</p>
								{/if}
								{#if restoreJob.manifest}
									<p class="text-muted-foreground mt-2">
										{restoreJob.manifest.deviceID || 'backup'} · {restoreJob.manifest.components?.join(', ') || 'manifest ready'}
									</p>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			</div>
		</section>

		<Separator />

		<section class="grid gap-5 py-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">Allowed Users</h2>
					<p class="text-muted-foreground text-sm">초대 목록은 Cloudflare Access와 Blueclaw policy의 기준 이메일입니다.</p>
				</div>
				<Badge variant="outline">{userEmails.length} users</Badge>
			</div>

			{#if !isDeviceContext()}
				<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
					초대 목록은 등록된 기기 주소에서 관리할 수 있습니다.
				</p>
			{:else}
				<form
					class="flex gap-2"
					onsubmit={(event) => {
						event.preventDefault();
						addEmail();
					}}
				>
					<div class="relative flex-1">
						<MailIcon class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
						<Input bind:value={newEmail} type="email" placeholder="name@company.com" class="pl-9" />
					</div>
					<Button type="submit" disabled={isSavingUser || !newEmail.trim()} class="gap-2">
						{#if isSavingUser}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<PlusIcon class="size-4" />
						{/if}
						Invite
					</Button>
				</form>

				{#if errorMessage}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
				{/if}

				{#if isLoadingUsers}
					<p class="text-muted-foreground text-sm">초대 목록을 불러오는 중...</p>
				{:else if userEmails.length === 0}
					<p class="text-muted-foreground text-sm">등록된 사용자가 없습니다.</p>
				{:else}
					<div class="overflow-hidden rounded-lg border">
						{#each userEmails as email, index}
							<div class="flex items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">{email}</p>
									{#if index === 0}
										<p class="text-muted-foreground text-xs">initial admin</p>
									{/if}
								</div>
								{#if index === 0}
									<Badge variant="secondary">admin</Badge>
								{:else}
									<Button variant="ghost" size="icon" disabled={isSavingUser} onclick={() => removeEmail(email)}>
										<XIcon class="size-4" />
									</Button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		</section>
	</div>
</main>
