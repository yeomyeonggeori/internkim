<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import { Textarea } from '$lib/components/ui/textarea';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
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
	import { adminText } from './admin/text';

	type UserRole = 'admin' | 'member';
	type AdminSection = 'device' | 'calendar' | 'flow' | 'bot' | 'companion' | 'backup' | 'users';

	type UserRecord = {
		userID: string;
		handle: string;
		name?: string;
		email: string;
		role: UserRole;
		mattermostUserID?: string;
		mattermostUsername?: string;
		status?: string;
		isIncomplete?: boolean;
	};

	type UsersResponse = {
		users?: string[];
		records?: UserRecord[];
		temporaryPassword?: string;
		temporaryPasswordEmail?: string;
	};

	type AdminSession = {
		email: string;
		claimedAdminEmail: string;
		isAdmin: boolean;
		isClaimed: boolean;
		bootstrapStatus: string;
		bootstrapError?: string;
		temporaryPassword?: string;
		temporaryPasswordEmail?: string;
	};

	type BackupManifest = {
		fleetID?: string;
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

	type BotProfile = {
		username: string;
		displayName: string;
		englishDisplayName?: string;
		aliases?: string[];
		publicDescription: string;
		identityExtension?: string;
	};

	const logoSrc = '/logo.svg';
	const storedFleetIdKey = 'internkim_fleet_id';

	let fleetIdInput = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let newHandle = $state('');
	let newName = $state('');
	let newEmail = $state('');
	let newUserRole = $state<UserRole>('member');
	let temporaryPasswordResult = $state<{ email: string; password: string } | null>(null);
	let isLoadingUsers = $state(false);
	let isSavingUser = $state(false);
	let errorMessage = $state('');
	let adminSession = $state<AdminSession | null>(null);
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
	let botProfile = $state<BotProfile>({
		username: 'internkim',
		displayName: '김인턴',
		englishDisplayName: 'Intern Kim',
		aliases: ['인턴킴', 'intern kim'],
		publicDescription: '',
		identityExtension: ''
	});
	let botProfileAliasesText = $state('인턴킴\nintern kim');
	let botProfileErrorMessage = $state('');
	let isLoadingBotProfile = $state(false);
	let isSavingBotProfile = $state(false);
	let activeAdminSection = $state<AdminSection>('device');
	const text = createPageText(adminText);

	const fleetId = () => {
		const explicitId = fleetIdInput.trim().toLowerCase();
		if (explicitId) return explicitId;
		if (!browser) return '';
		return fleetIdFromHost();
	};

	const mattermostURL = () => {
		const id = fleetId();
		return id ? `https://${id}.example.test` : '';
	};

	const isDeviceContext = () => fleetId() !== '';
	const adminBaseURL = () => {
		const id = fleetId();
		if (fleetIdFromHost()) return '/admin/api';
		return id ? `https://${id}.example.test/admin/api` : '';
	};
	const usersBaseURL = () => adminBaseURL();
	const companionReleaseURL = () => {
		if (fleetIdFromHost()) return '/admin/api/companion/releases';
		return '/api/companion/releases';
	};
	const backupDownloadURL = () => {
		if (!backupJob?.downloadURL || !fleetId()) return '';
		if (fleetIdFromHost()) return backupJob.downloadURL;
		return `https://${fleetId()}.example.test${backupJob.downloadURL}`;
	};
	const userCount = () => userRecords.length;
	const adminCount = () => userRecords.filter((record) => record.role === 'admin').length;
	const normalizeHandle = (handle: string) => handle.trim().toLowerCase();
	const isValidHandle = (handle: string) => /^[a-z][a-z0-9._-]{2,21}$/.test(normalizeHandle(handle));
	const isValidUserRecord = (record: UserRecord) => isValidHandle(record.handle) && !!record.name?.trim() && !!record.email.trim();
	const adminSessionStatusText = () => {
		if (!adminSession) return '';
		if (adminSession.isAdmin) return 'admin';
		if (adminSession.bootstrapStatus === 'identity_missing') return 'Access email missing';
		if (adminSession.bootstrapStatus === 'failed') return 'claim failed';
		if (adminSession.bootstrapStatus === 'rejected') return 'not admin';
		if (!adminSession.isClaimed) return 'first admin claim pending';
		return 'not admin';
	};
	const adminSections = (): { value: AdminSection; label: string }[] => [
		{ value: 'device', label: text.sections.device },
		{ value: 'calendar', label: text.sections.calendar },
		{ value: 'flow', label: text.sections.flow },
		{ value: 'users', label: text.sections.users },
		{ value: 'companion', label: text.sections.companion },
		{ value: 'backup', label: text.sections.backup },
		{ value: 'bot', label: text.sections.bot }
	];
	const userRoleOptions = () => [
		{ value: 'member', label: text.users.member },
		{ value: 'admin', label: text.users.admin }
	];

	onMount(() => {
		const queryFleetId = new URLSearchParams(location.search).get('fleet_id')?.trim().toLowerCase() ?? '';
		if (queryFleetId && !fleetIdFromHost() && !isLocalBrowserHost()) {
			location.replace(`https://${queryFleetId}.example.test/admin/`);
			return;
		}
		fleetIdInput = queryFleetId || fleetIdFromHost() || localStorage.getItem(storedFleetIdKey) || '';
		if (fleetIdInput) localStorage.setItem(storedFleetIdKey, fleetIdInput);
		loadCompanionReleases();
		loadAdminSession();
		loadUsers();
		checkDevice();
		loadBotProfile();
	});

	function fleetIdFromHost() {
		if (!browser) return '';
		const host = location.hostname;
		if (isLocalBrowserHost()) return '';
		const suffix = '.example.test';
		if (!host.endsWith(suffix)) return '';
		const id = host.slice(0, -suffix.length);
		if (!id || id === 'api' || id.includes('.')) return '';
		return id;
	}

	function isLocalBrowserHost() {
		if (!browser) return false;
		const host = location.hostname;
		return host === 'localhost' || host === '127.0.0.1' || /^\d+\.\d+\.\d+\.\d+$/.test(host);
	}

	function saveFleetId() {
		fleetIdInput = fleetIdInput.trim().toLowerCase();
		if (fleetIdInput) localStorage.setItem(storedFleetIdKey, fleetIdInput);
		loadUsers();
		checkDevice();
		loadCompanions();
		loadAdminSession();
		loadBotProfile();
	}

	async function loadAdminSession() {
		if (!adminBaseURL()) return;
		try {
			const response = await fetch(`${adminBaseURL()}/session`, { credentials: 'include' });
			if (!response.ok) return;
			adminSession = (await response.json()) as AdminSession;
			if (adminSession.temporaryPassword && adminSession.temporaryPasswordEmail) {
				temporaryPasswordResult = {
					email: adminSession.temporaryPasswordEmail,
					password: adminSession.temporaryPassword
				};
			}
		} catch {
			adminSession = null;
		}
	}

	async function loadBotProfile() {
		if (!adminBaseURL()) return;

		isLoadingBotProfile = true;
		botProfileErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/bot-profile`, { credentials: 'include' });
			if (!response.ok) {
				botProfileErrorMessage = '봇 프로필을 불러오지 못했습니다.';
				return;
			}
			applyBotProfile((await response.json()) as BotProfile);
		} catch {
			botProfileErrorMessage = '봇 프로필을 불러오지 못했습니다.';
		} finally {
			isLoadingBotProfile = false;
		}
	}

	function applyBotProfile(profile: BotProfile) {
		botProfile = {
			username: profile.username || 'internkim',
			displayName: profile.displayName || '김인턴',
			englishDisplayName: profile.englishDisplayName || 'Intern Kim',
			aliases: profile.aliases ?? [],
			publicDescription: profile.publicDescription || '',
			identityExtension: profile.identityExtension || ''
		};
		botProfileAliasesText = (botProfile.aliases ?? []).join('\n');
	}

	async function saveBotProfile() {
		if (!adminBaseURL()) return;

		isSavingBotProfile = true;
		botProfileErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/bot-profile`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					...botProfile,
					username: 'internkim',
					aliases: botProfileAliasesText
						.split('\n')
						.map((alias) => alias.trim())
						.filter(Boolean)
				})
			});
			if (!response.ok) {
				botProfileErrorMessage = await response.text();
				return;
			}
			applyBotProfile((await response.json()) as BotProfile);
		} catch {
			botProfileErrorMessage = '봇 프로필 저장에 실패했습니다.';
		} finally {
			isSavingBotProfile = false;
		}
	}

	async function loadCompanionReleases() {
		try {
			const response = await fetch(companionReleaseURL(), { credentials: 'include' });
			if (!response.ok) return;
			const data = (await response.json()) as CompanionReleaseResponse;
			companionReleases = data.platforms ?? [];
		} catch {
			companionReleases = [];
		}
	}

	async function loadUsers() {
		const id = fleetId();
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
			applyUsersResponse(data);
		} catch {
			errorMessage = '초대 목록을 불러오지 못했습니다.';
		} finally {
			isLoadingUsers = false;
		}
	}

	function applyUsersResponse(data: UsersResponse) {
		if (data.records) {
			userRecords = data.records.map((record) => ({ ...record, name: record.name ?? '' }));
		} else {
			userRecords = (data.users ?? []).map((email, index) => ({
				userID: `legacy-${index}`,
				handle: email.split('@')[0]?.toLowerCase() ?? '',
				email,
				role: index === 0 ? 'admin' : 'member'
			}));
		}
		if (data.temporaryPassword && data.temporaryPasswordEmail) {
			temporaryPasswordResult = {
				email: data.temporaryPasswordEmail,
				password: data.temporaryPassword
			};
		}
	}

	async function addEmail() {
		const email = newEmail.trim().toLowerCase();
		const handle = normalizeHandle(newHandle);
		const name = newName.trim();
		if (!email || !handle || !name || !fleetId()) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			const response = await fetch(`${usersBaseURL()}/users`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ handle, name, email, role: newUserRole })
			});
			if (!response.ok) {
				const detail = (await response.text()).trim();
				errorMessage = response.status === 403
					? '관리자 인증이 필요합니다. 기기 주소의 /admin에서 Cloudflare Access로 로그인해 주세요.'
					: detail || '사용자 초대에 실패했습니다.';
				return;
			}
			const data = (await response.json()) as UsersResponse;
			applyUsersResponse(data);
			await loadAdminSession();
			newHandle = '';
			newName = '';
			newEmail = '';
			newUserRole = 'member';
		} catch {
			errorMessage = '사용자 초대에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}

	async function saveUser(record: UserRecord, role: UserRole = record.role) {
		if (!fleetId()) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			const response = await fetch(`${usersBaseURL()}/users`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					userID: record.userID,
					handle: normalizeHandle(record.handle),
					name: record.name?.trim() ?? '',
					email: record.email,
					role,
					mattermostUserID: record.mattermostUserID,
					mattermostUsername: record.mattermostUsername,
					status: record.status
				})
			});
			if (!response.ok) {
				const detail = (await response.text()).trim();
				errorMessage = response.status === 403 ? '관리자 인증이 필요합니다.' : detail || '사용자 저장에 실패했습니다.';
				return;
			}
			applyUsersResponse((await response.json()) as UsersResponse);
		} catch {
			errorMessage = '사용자 저장에 실패했습니다.';
		} finally {
			isSavingUser = false;
		}
	}

	async function removeEmail(email: string) {
		if (!fleetId()) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
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
			applyUsersResponse(data);
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
	<title>{text.title}</title>
</svelte:head>

<main class="bg-background text-foreground min-h-svh">
	<div class="mx-auto flex min-h-svh w-full max-w-4xl flex-col px-5 py-6">
		<header class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<img src={logoSrc} alt="intern kim" class="size-9" />
				<div>
					<h1 class="text-lg font-semibold leading-tight">intern kim</h1>
					<p class="text-muted-foreground text-sm">{text.subtitle}</p>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<Badge variant="secondary" class="gap-1.5">
					<ShieldCheckIcon class="size-3.5" />
					{text.accessProtected}
				</Badge>
			</div>
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
					<h2 class="text-2xl font-semibold">{text.heroTitle}</h2>
					<p class="text-muted-foreground mt-2 text-sm leading-6">
						{text.heroDescription}
					</p>
				</div>
				{#if mattermostURL()}
					<div class="flex flex-wrap items-center gap-2">
						<Button href={mattermostURL()} class="gap-2">
							<ExternalLinkIcon class="size-4" />
							{text.openMattermost}
						</Button>
						<CopyButton text={mattermostURL()} variant="outline" />
					</div>
				{:else}
					<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
						{text.devicePending}
					</p>
				{/if}
			</div>
		</section>

		<Separator />

		<nav class="flex gap-1 overflow-x-auto py-4">
			{#each adminSections() as section}
				<Button
					variant={activeAdminSection === section.value ? 'default' : 'ghost'}
					size="sm"
					onclick={() => (activeAdminSection = section.value)}
				>
					{section.label}
				</Button>
			{/each}
		</nav>

		<section class="grid gap-5 py-6">
			{#if activeAdminSection === 'device'}
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h2 class="text-base font-semibold">{text.device.title}</h2>
						<p class="text-muted-foreground text-sm">{text.device.description}</p>
						{#if adminSession?.email}
							<p class="text-muted-foreground mt-1 text-xs">
								Cloudflare Access: <span class="font-medium text-foreground">{adminSession.email}</span>
								{#if adminSession.isAdmin}
									<span class="ml-1 text-emerald-700">admin</span>
								{:else if adminSession.bootstrapStatus === 'failed'}
									<span class="ml-1 text-destructive">claim failed</span>
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
			{/if}

			{#if activeAdminSection === 'flow'}
				<div class="rounded-lg border p-4">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">{text.flow.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							{text.flow.description}
						</p>
					</div>
					<Badge variant="secondary">weekly</Badge>
				</div>
				<div class="mt-4 flex flex-wrap gap-2">
					<Button href="/flow/" class="gap-2">
						<ExternalLinkIcon class="size-4" />
						{text.flow.open}
					</Button>
					<Button href="/admin/api/flow/status" variant="outline" class="gap-2">
						<RefreshCwIcon class="size-4" />
						{text.flow.status}
					</Button>
				</div>
			</div>
			{/if}

			{#if activeAdminSection === 'calendar'}
				<div class="rounded-lg border p-4">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">{text.calendar.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							{text.calendar.description}
						</p>
					</div>
					<Badge variant="secondary">CalDAV</Badge>
				</div>
				<div class="mt-4 flex flex-wrap gap-2">
					<Button href="/calendar/" class="gap-2">
						<CalendarDaysIcon class="size-4" />
						{text.calendar.open}
					</Button>
				</div>
			</div>
			{/if}

			{#if activeAdminSection === 'bot'}
				<div class="rounded-lg border p-4">
				<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">{text.bot.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							{text.bot.description}
						</p>
					</div>
					<Badge variant="outline">{botProfile.username}</Badge>
				</div>
				<div class="grid gap-3 md:grid-cols-2">
					<Input bind:value={botProfile.displayName} placeholder="display name" disabled={isLoadingBotProfile} />
					<Input bind:value={botProfile.englishDisplayName} placeholder="English display name" disabled={isLoadingBotProfile} />
					<Input
						class="md:col-span-2"
						bind:value={botProfile.publicDescription}
						placeholder="public description"
						disabled={isLoadingBotProfile}
					/>
					<Textarea
						bind:value={botProfileAliasesText}
						placeholder="aliases, one per line"
						disabled={isLoadingBotProfile}
						class="min-h-24"
					/>
					<Textarea
						bind:value={botProfile.identityExtension}
						placeholder="prompt-only identity extension"
						disabled={isLoadingBotProfile}
						class="min-h-24"
					/>
				</div>
				<div class="mt-3 flex flex-wrap items-center justify-between gap-3">
					<p class="text-muted-foreground text-xs">
						{text.bot.identityNotice}
					</p>
					<Button disabled={!isDeviceReachable || isSavingBotProfile || !botProfile.displayName.trim()} onclick={saveBotProfile}>
						{#if isSavingBotProfile}
							<LoaderIcon class="size-4 animate-spin" />
						{/if}
						{text.bot.save}
					</Button>
				</div>
				{#if botProfileErrorMessage}
					<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{botProfileErrorMessage}
					</p>
				{/if}
			</div>
			{/if}

			{#if activeAdminSection === 'companion'}
				<div class="rounded-lg border p-4">
				<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">{text.companion.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							{text.companion.description}
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
									{text.companion.download}
								</Button>
							{:else}
								<Button disabled variant="outline" class="gap-2">
									<DownloadIcon class="size-4" />
									{text.companion.betaComingSoon}
								</Button>
							{/if}
						{:else}
							<p class="text-muted-foreground text-sm">{text.companion.loadingDownloads}</p>
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
							<p class="text-sm font-medium">{text.companion.connectTitle}</p>
							<p class="text-muted-foreground text-xs">{text.companion.connectDescription}</p>
						</div>
						<Button disabled={!isDeviceReachable || isCreatingPairingCode} onclick={createCompanionPairingCode} class="gap-2">
							{#if isCreatingPairingCode}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<ExternalLinkIcon class="size-4" />
							{/if}
							{text.companion.connect}
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
						<p class="text-muted-foreground p-3 text-sm">{text.companion.loading}</p>
					{:else if companionStatuses.length === 0}
						<p class="text-muted-foreground p-3 text-sm">{text.companion.empty}</p>
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
										{companion.capabilities?.map((capability) => capability.name).join(', ') || text.companion.noCapabilities}
									</p>
									{#if companion.isOnline && !companion.capabilities?.some((capability) => capability.name.startsWith('browser.'))}
										<p class="mt-1 text-xs text-destructive">{text.companion.browserUnavailable}</p>
									{/if}
								</div>
								<Button variant="ghost" size="sm" onclick={() => revokeCompanion(companion.companionID)}>{text.companion.revoke}</Button>
							</div>
						{/each}
					{/if}
				</div>
			</div>
			{/if}

			{#if activeAdminSection === 'backup'}
				<div class="grid gap-4 md:grid-cols-2">
				<div class="rounded-lg border p-4">
					<div class="mb-4">
						<h3 class="text-sm font-semibold">{text.backup.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">{text.backup.description}</p>
					</div>
					<div class="grid gap-3">
						<Input bind:value={backupPassphrase} type="password" placeholder="backup passphrase" autocomplete="new-password" />
						<Button disabled={!isDeviceReachable || isCreatingBackup || !backupPassphrase.trim()} onclick={createBackup} class="gap-2">
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
										{backupJob.manifest.fleetID || fleetId()} · {backupJob.manifest.components?.join(', ') || 'manifest ready'}
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
										{restoreJob.manifest.fleetID || 'backup'} · {restoreJob.manifest.components?.join(', ') || 'manifest ready'}
									</p>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			</div>
			{/if}
		</section>

		{#if activeAdminSection === 'users'}
			<Separator />

			<section class="grid gap-5 py-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">{text.users.title}</h2>
					<p class="text-muted-foreground text-sm">
						{text.users.description}
					</p>
				</div>
				<Badge variant="outline">{userCount()} users</Badge>
			</div>

			{#if !isDeviceContext()}
				<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
					{text.users.deviceOnly}
				</p>
			{:else}
				<form
					class="grid gap-2 md:grid-cols-[140px_1fr_1fr_120px_auto]"
					onsubmit={(event) => {
						event.preventDefault();
						addEmail();
					}}
				>
					<Input bind:value={newHandle} placeholder="handle" autocomplete="off" />
					<Input bind:value={newName} placeholder="real name" autocomplete="off" />
					<div class="relative">
						<MailIcon class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
						<Input bind:value={newEmail} type="email" placeholder="name@company.com" class="pl-9" />
					</div>
					<Select.Root type="single" bind:value={newUserRole}>
						<Select.Trigger class="w-full">
							{userRoleOptions().find((option) => option.value === newUserRole)?.label ?? '-'}
						</Select.Trigger>
						<Select.Content>
							{#each userRoleOptions() as option (option.value)}
								<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					<Button type="submit" disabled={isSavingUser || !newEmail.trim() || !newName.trim() || !isValidHandle(newHandle)} class="gap-2">
						{#if isSavingUser}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<PlusIcon class="size-4" />
						{/if}
						{text.users.invite}
					</Button>
				</form>

				<p class="text-muted-foreground text-sm">
					{text.users.passwordNotice}
				</p>

				{#if temporaryPasswordResult}
					<div class="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950">
						<div class="flex flex-wrap items-start justify-between gap-3">
							<div>
								<p class="font-semibold">{text.users.temporaryPasswordTitle}: {temporaryPasswordResult.email} / {temporaryPasswordResult.password}</p>
								<p class="mt-1">{text.users.temporaryPasswordNotice}</p>
								<code class="mt-3 block rounded-md bg-white px-3 py-2 font-mono text-base">{temporaryPasswordResult.password}</code>
							</div>
							<CopyButton text={temporaryPasswordResult.password} />
						</div>
					</div>
				{/if}

				{#if errorMessage}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
				{/if}

				{#if isLoadingUsers}
					<p class="text-muted-foreground text-sm">{text.users.loading}</p>
				{:else if userRecords.length === 0}
					<p class="text-muted-foreground text-sm">{text.users.empty}</p>
				{:else}
					<div class="overflow-hidden rounded-lg border">
						{#each userRecords as record}
							<div class="grid gap-3 border-b px-3 py-3 last:border-b-0 md:grid-cols-[140px_1fr_1fr_auto] md:items-center">
								<Input bind:value={record.handle} placeholder="handle" autocomplete="off" />
								<Input bind:value={record.name} placeholder="real name" autocomplete="off" />
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">{record.email}</p>
									{#if record.mattermostUsername}
										<p class="text-muted-foreground text-xs">Mattermost: {record.mattermostUsername}</p>
									{/if}
									{#if record.isIncomplete}
										<p class="text-destructive text-xs">{text.users.incomplete}</p>
									{/if}
								</div>
								<div class="flex flex-wrap items-center gap-2">
									<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{record.role}</Badge>
									<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => saveUser(record)}>
										{text.users.save}
									</Button>
									{#if record.role === 'admin'}
										<Button
											variant="outline"
											size="sm"
											disabled={isSavingUser || adminCount() <= 1 || !isValidUserRecord(record)}
											onclick={() => saveUser(record, 'member')}
										>
											{text.users.makeMember}
										</Button>
									{:else}
										<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => saveUser(record, 'admin')}>
											{text.users.makeAdmin}
										</Button>
									{/if}
									<Button
										variant="ghost"
										size="icon"
										disabled={isSavingUser || (record.role === 'admin' && adminCount() <= 1)}
										onclick={() => removeEmail(record.email)}
									>
										<XIcon class="size-4" />
									</Button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			{/if}
			</section>
		{/if}
	</div>
</main>
