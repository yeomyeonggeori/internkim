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

	type CompanionCapability = {
		name: string;
		version?: string;
		privacyClass?: string;
		requiresUserPresence?: boolean;
		worksOffline?: boolean;
	};

	type CompanionStatus = {
		companionID: string;
		displayName: string;
		capabilities?: CompanionCapability[];
		localOnly?: boolean;
		isOnline?: boolean;
		lastSeenAt?: string;
		disabled?: boolean;
	};

	type CompanionStatusResponse = {
		companions?: CompanionStatus[];
	};

	type CompanionPairingCodeResponse = {
		code: string;
		expiresAt: string;
		deepLink: string;
	};

	type CompanionRelease = {
		platform: string;
		label: string;
		architecture: string;
		status?: string;
		url: string;
	};

	type CompanionReleaseResponse = {
		platforms?: CompanionRelease[];
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
	let companionStatuses = $state<CompanionStatus[]>([]);
	let companionReleases = $state<CompanionRelease[]>([]);
	let companionPairingCode = $state<CompanionPairingCodeResponse | null>(null);
	let companionErrorMessage = $state('');
	let isLoadingCompanions = $state(false);
	let isCreatingPairingCode = $state(false);

	const deviceId = () => {
		const explicitId = deviceIdInput.trim().toLowerCase();
		if (explicitId) return explicitId;
		if (!browser) return '';
		return deviceIdFromHost();
	};

	const mattermostURL = () => {
		const id = deviceId();
		return id ? `https://${id}.example.test` : '';
	};

	const isDeviceContext = () => deviceId() !== '';
	const adminBaseURL = () => {
		const id = deviceId();
		return id ? `https://${id}.example.test/_internkim/admin` : '';
	};
	const usersBaseURL = () => {
		if (!browser) return '';
		if (deviceIdFromHost()) return '/_internkim/admin';
		return adminBaseURL();
	};
	const backupDownloadURL = () => {
		if (!backupJob?.downloadURL || !deviceId()) return '';
		return `https://${deviceId()}.example.test${backupJob.downloadURL}`;
	};

	onMount(() => {
		const queryDeviceId = new URLSearchParams(location.search).get('device_id')?.trim().toLowerCase() ?? '';
		if (queryDeviceId && location.hostname === 'api.example.test') {
			location.replace(`https://${queryDeviceId}.example.test/admin/`);
			return;
		}
		deviceIdInput = queryDeviceId || deviceIdFromHost() || localStorage.getItem(storedDeviceIdKey) || '';
		if (deviceIdInput) localStorage.setItem(storedDeviceIdKey, deviceIdInput);
		loadCompanionReleases();
		loadUsers();
		checkDevice();
	});

	function deviceIdFromHost() {
		if (!browser) return '';
		const host = location.hostname;
		if (host === 'localhost' || host === '127.0.0.1' || /^\d+\.\d+\.\d+\.\d+$/.test(host)) return '';
		const suffix = '.example.test';
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
		loadCompanions();
	}

	async function loadCompanionReleases() {
		try {
			const response = await fetch('https://api.example.test/api/companion/releases');
			if (!response.ok) return;
			const data = (await response.json()) as CompanionReleaseResponse;
			companionReleases = data.platforms ?? [];
		} catch {
			companionReleases = [];
		}
	}

	async function loadUsers() {
		const id = deviceId();
		if (!id) return;

		isLoadingUsers = true;
		errorMessage = '';
		try {
			const response = await fetch(`${usersBaseURL()}/users`, { credentials: 'include' });
			if (!response.ok) {
				errorMessage =
					response.status === 403
						? '관리자 인증이 필요합니다. 기기 주소의 /admin에서 Cloudflare Access로 로그인해 주세요.'
						: '초대 목록을 불러오지 못했습니다.';
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
			const response = await fetch(`${usersBaseURL()}/users`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email })
			});
			if (!response.ok) {
				errorMessage =
					response.status === 403
						? '관리자 인증이 필요합니다. 기기 주소의 /admin에서 Cloudflare Access로 로그인해 주세요.'
						: '사용자 초대에 실패했습니다.';
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
			const response = await fetch(`${usersBaseURL()}/users/${encodeURIComponent(email)}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) {
				errorMessage =
					response.status === 403
						? '관리자 인증이 필요합니다. 기기 주소의 /admin에서 Cloudflare Access로 로그인해 주세요.'
						: '사용자 제거에 실패했습니다.';
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
		if (isDeviceReachable) await loadCompanions();
	}

	async function loadCompanions() {
		if (!adminBaseURL()) return;

		isLoadingCompanions = true;
		companionErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/companion/status`, { credentials: 'include' });
			if (!response.ok) {
				companionErrorMessage = 'Companion 상태를 불러오지 못했습니다.';
				return;
			}
			const data = (await response.json()) as CompanionStatusResponse;
			companionStatuses = data.companions ?? [];
		} catch {
			companionErrorMessage = 'Companion 상태를 불러오지 못했습니다.';
		} finally {
			isLoadingCompanions = false;
		}
	}

	async function createCompanionPairingCode() {
		if (!adminBaseURL()) return;

		isCreatingPairingCode = true;
		companionErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/companion/pairing-codes`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!response.ok) {
				companionErrorMessage = '연결 코드를 만들지 못했습니다.';
				return;
			}
			companionPairingCode = (await response.json()) as CompanionPairingCodeResponse;
			if (companionPairingCode.deepLink && browser) location.href = companionPairingCode.deepLink;
		} catch {
			companionErrorMessage = '연결 코드를 만들지 못했습니다.';
		} finally {
			isCreatingPairingCode = false;
		}
	}

	async function revokeCompanion(companionID: string) {
		if (!adminBaseURL()) return;

		companionErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/companion/${encodeURIComponent(companionID)}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) {
				companionErrorMessage = 'Companion 연결을 해제하지 못했습니다.';
				return;
			}
			await loadCompanions();
		} catch {
			companionErrorMessage = 'Companion 연결을 해제하지 못했습니다.';
		}
	}

	function detectedCompanionPlatform() {
		if (!browser) return 'macos';
		const userAgent = navigator.userAgent.toLowerCase();
		if (userAgent.includes('windows')) return 'windows';
		if (userAgent.includes('linux')) return 'linux';
		return 'macos';
	}

	function recommendedCompanionRelease() {
		const platform = detectedCompanionPlatform();
		return companionReleases.find((release) => release.platform === platform) ?? companionReleases[0];
	}

	function isCompanionReleaseAvailable(release: CompanionRelease | undefined) {
		return !!release?.url && release.status !== 'coming_soon';
	}

	function onlineCompanionCount() {
		return companionStatuses.filter((companion) => companion.isOnline && !companion.disabled).length;
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

			<div class="rounded-lg border p-4">
				<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">Companion App</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							사용자 컴퓨터에서 브라우저, 파일 선택, 확인 요청, 로컬 실행을 맡는 작은 앱입니다.
						</p>
					</div>
					<Badge variant={onlineCompanionCount() > 0 ? 'secondary' : 'outline'}>{onlineCompanionCount()} online</Badge>
				</div>

				<div class="grid gap-4 lg:grid-cols-[1fr_1fr]">
					<div class="grid gap-3 rounded-md bg-muted/30 p-3">
						{#if recommendedCompanionRelease()}
							<div>
								<p class="text-sm font-medium">{recommendedCompanionRelease()?.label} companion</p>
								<p class="text-muted-foreground text-xs">
									{recommendedCompanionRelease()?.architecture}
									{#if !isCompanionReleaseAvailable(recommendedCompanionRelease())}
										· coming soon
									{/if}
								</p>
							</div>
							{#if isCompanionReleaseAvailable(recommendedCompanionRelease())}
								<Button href={recommendedCompanionRelease()?.url} variant="outline" class="gap-2">
									<DownloadIcon class="size-4" />
									Download companion
								</Button>
							{:else}
								<Button disabled variant="outline" class="gap-2">
									<DownloadIcon class="size-4" />
									Beta build coming soon
								</Button>
							{/if}
						{:else}
							<p class="text-muted-foreground text-sm">다운로드 정보를 불러오는 중...</p>
						{/if}
						{#if companionReleases.length > 1}
							<div class="flex flex-wrap gap-2">
								{#each companionReleases as release}
									{#if isCompanionReleaseAvailable(release)}
										<Button href={release.url} variant="ghost" size="sm">{release.label}</Button>
									{:else}
										<Button disabled variant="ghost" size="sm">{release.label} soon</Button>
									{/if}
								{/each}
							</div>
						{/if}
					</div>

					<div class="grid gap-3 rounded-md bg-muted/30 p-3">
						<div>
							<p class="text-sm font-medium">Connect to this Intern Kim</p>
							<p class="text-muted-foreground text-xs">연결 코드는 10분 동안 한 번만 사용할 수 있습니다.</p>
						</div>
						<Button disabled={!isDeviceReachable || isCreatingPairingCode} onclick={createCompanionPairingCode} class="gap-2">
							{#if isCreatingPairingCode}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<ExternalLinkIcon class="size-4" />
							{/if}
							Connect
						</Button>
						{#if companionPairingCode}
							<div class="rounded-md border bg-background p-3 text-sm">
								<p class="font-medium">{companionPairingCode.code}</p>
								<p class="text-muted-foreground mt-1 text-xs">
									expires {new Date(companionPairingCode.expiresAt).toLocaleTimeString()}
								</p>
								<div class="mt-3 flex flex-wrap gap-2">
									<CopyButton text={companionPairingCode.code} variant="outline" />
									<CopyButton
										text={`internkim-companion pair --device-url ${mattermostURL()} --code ${companionPairingCode.code}`}
										variant="outline"
									/>
								</div>
							</div>
						{/if}
					</div>
				</div>

				{#if companionErrorMessage}
					<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{companionErrorMessage}
					</p>
				{/if}

				<div class="mt-4 overflow-hidden rounded-lg border">
					{#if isLoadingCompanions}
						<p class="text-muted-foreground p-3 text-sm">Companion 상태를 불러오는 중...</p>
					{:else if companionStatuses.length === 0}
						<p class="text-muted-foreground p-3 text-sm">아직 연결된 Companion이 없습니다.</p>
					{:else}
						{#each companionStatuses as companion}
							<div class="flex flex-wrap items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0">
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="truncate text-sm font-medium">{companion.displayName || companion.companionID}</p>
										<Badge variant={companion.isOnline ? 'secondary' : 'outline'}>{companion.isOnline ? 'online' : 'offline'}</Badge>
										{#if companion.localOnly}
											<Badge variant="outline">local only</Badge>
										{/if}
									</div>
									<p class="text-muted-foreground mt-1 truncate text-xs">
										{companion.capabilities?.map((capability) => capability.name).join(', ') || 'no capabilities'}
									</p>
									{#if companion.isOnline && !companion.capabilities?.some((capability) => capability.name.startsWith('browser.'))}
										<p class="mt-1 text-xs text-destructive">browser runtime unavailable</p>
									{/if}
								</div>
								<Button variant="ghost" size="sm" onclick={() => revokeCompanion(companion.companionID)}>Revoke</Button>
							</div>
						{/each}
					{/if}
				</div>
			</div>

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
