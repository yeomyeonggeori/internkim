<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import * as Table from '$lib/components/ui/table';
	import { Textarea } from '$lib/components/ui/textarea';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
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
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import QrCode from 'svelte-qrcode';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { adminText } from './admin/text';

	type UserRole = 'admin' | 'member';
	type WorkspaceLanguage = 'ko' | 'en';
	type AdminSection = 'device' | 'bot' | 'credentials' | 'companion' | 'backup' | 'users' | 'settings';

	type UserRecord = {
		userID: string;
		handle: string;
		name?: string;
		email: string;
		hireDate?: string;
		role: UserRole;
		circles?: string[];
		mattermostUserID?: string;
		mattermostUsername?: string;
		status?: string;
		isIncomplete?: boolean;
	};

	type CircleRecord = {
		circleID: string;
		displayName: string;
		isMattermostManaged?: boolean;
	};

	type UsersResponse = {
		users?: string[];
		records?: UserRecord[];
		availableCircles?: CircleRecord[];
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

	type ReleaseUpdateSummary = {
		releaseID: string;
		channel?: string;
		createdAt?: string;
		components?: Record<string, { revision: string; sha256?: string }>;
	};

	type BlueclawUpdateStatus = {
		current?: ReleaseUpdateSummary;
		latest?: ReleaseUpdateSummary;
		state: string;
		updateAllowed: boolean;
		activeJob?: AdminJob;
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

	type CredentialProviderStatus = {
		provider: string;
		configured: boolean;
		fingerprint?: string;
	};

	type CredentialProvidersResponse = {
		providers?: CredentialProviderStatus[];
	};

	type WorkspaceSettings = {
		timeZone: string;
		language: WorkspaceLanguage;
		updatedAt?: string;
	};

	type AttendanceLocation = {
		id: string;
		name: string;
		color: string;
		isDefault: boolean;
		updatedAt?: string;
	};

	type AttendanceLocationsResponse = {
		locations?: AttendanceLocation[];
	};

	const logoSrc = '/logo.svg';
	const storedFleetIdKey = 'internkim_fleet_id';

	let fleetIdInput = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let availableCircles = $state<CircleRecord[]>([{ circleID: 'staff', displayName: 'Staff' }]);
	let newHandle = $state('');
	let newName = $state('');
	let newEmail = $state('');
	let newHireDate = $state('');
	let newUserRole = $state<UserRole>('member');
	let newCircleID = $state('');
	let newCircleName = $state('');
	let newCircleMattermostManaged = $state(true);
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
	let blueclawUpdateStatus = $state<BlueclawUpdateStatus | null>(null);
	let blueclawUpdateJob = $state<AdminJob | null>(null);
	let blueclawUpdateMessage = $state('');
	let isLoadingBlueclawUpdate = $state(false);
	let isApplyingBlueclawUpdate = $state(false);
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
	let credentialProviders = $state<CredentialProviderStatus[]>([]);
	let openRouterApiKey = $state('');
	let credentialErrorMessage = $state('');
	let isLoadingCredentials = $state(false);
	let isSavingCredential = $state(false);
	let workspaceSettings = $state<WorkspaceSettings>({ timeZone: 'system', language: 'ko' });
	let workspaceSettingsDraft = $state<WorkspaceSettings>({ timeZone: 'system', language: 'ko' });
	let workspaceSettingsMessage = $state('');
	let isLoadingWorkspaceSettings = $state(false);
	let isSavingWorkspaceSettings = $state(false);
	let attendanceLocations = $state<AttendanceLocation[]>([]);
	let attendanceLocationsMessage = $state('');
	let isLoadingAttendanceLocations = $state(false);
	let isSavingAttendanceLocations = $state(false);
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
		return id ? `https://${id}.intern.kim` : '';
	};

	const isDeviceContext = () => fleetId() !== '';
	const adminBaseURL = () => {
		const id = fleetId();
		if (fleetIdFromHost()) return '/admin/api';
		return id ? `https://${id}.intern.kim/admin/api` : '';
	};
	const usersBaseURL = () => adminBaseURL();
	const companionReleaseURL = () => {
		if (fleetIdFromHost()) return '/admin/api/companion/releases';
		return '/api/companion/releases';
	};
	const backupDownloadURL = () => {
		if (!backupJob?.downloadURL || !fleetId()) return '';
		if (fleetIdFromHost()) return backupJob.downloadURL;
		return `https://${fleetId()}.intern.kim${backupJob.downloadURL}`;
	};
	const userCount = () => userRecords.length;
	const adminCount = () => userRecords.filter((record) => record.role === 'admin').length;
	const normalizeHandle = (handle: string) => handle.trim().toLowerCase();
	const isValidHandle = (handle: string) => /^[a-z][a-z0-9._-]{2,21}$/.test(normalizeHandle(handle));
	const isValidUserRecord = (record: UserRecord) => isValidHandle(record.handle) && !!record.name?.trim() && !!record.email.trim();
	const userRecordsByHireDate = () =>
		[...userRecords].sort((first, second) => {
			if (first.hireDate || second.hireDate) {
				if (!first.hireDate) return 1;
				if (!second.hireDate) return -1;
				if (first.hireDate !== second.hireDate) return first.hireDate.localeCompare(second.hireDate);
			}
			return (first.name || first.email).localeCompare(second.name || second.email);
		});
	const adminSessionStatusText = () => {
		if (!adminSession) return '';
		if (adminSession.isAdmin) return text.device.admin;
		if (adminSession.bootstrapStatus === 'identity_missing') return text.device.accessEmailMissing;
		if (adminSession.bootstrapStatus === 'failed') return text.device.claimFailed;
		if (adminSession.bootstrapStatus === 'rejected') return text.device.notAdmin;
		if (!adminSession.isClaimed) return text.device.firstAdminClaimPending;
		return text.device.notAdmin;
	};
	const adminSections = (): { value: AdminSection; label: string }[] => [
		{ value: 'device', label: text.sections.device },
		{ value: 'users', label: text.sections.users },
		{ value: 'credentials', label: text.sections.credentials },
		{ value: 'companion', label: text.sections.companion },
		{ value: 'backup', label: text.sections.backup },
		{ value: 'bot', label: text.sections.bot },
		{ value: 'settings', label: text.sections.settings }
	];
	const userRoleOptions = () => [
		{ value: 'member', label: text.users.member },
		{ value: 'admin', label: text.users.admin }
	];
	const visibleCircles = () => availableCircles.filter((circle) => circle.circleID !== 'admin');
	const workspaceLanguageOptions = (): { value: WorkspaceLanguage; label: string }[] => [
		{ value: 'ko', label: text.settings.workspaceLanguageKorean },
		{ value: 'en', label: text.settings.workspaceLanguageEnglish }
	];
	const shortRevision = (revision: string | undefined) => {
		const value = revision?.trim() ?? '';
		return value ? value.slice(0, 8) : text.deviceUpdate.notAvailable;
	};
	const releaseComponents = (summary: ReleaseUpdateSummary | undefined) => {
		const components = summary?.components ?? {};
		return Object.entries(components).sort(([leftName], [rightName]) => leftName.localeCompare(rightName));
	};
	const blueclawUpdateStateLabel = (state: string | undefined) => {
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
	};

	onMount(() => {
		const queryFleetId = new URLSearchParams(location.search).get('fleet_id')?.trim().toLowerCase() ?? '';
		if (queryFleetId && !fleetIdFromHost() && !isLocalBrowserHost()) {
			location.replace(`https://${queryFleetId}.intern.kim/admin/`);
			return;
		}
		fleetIdInput = queryFleetId || fleetIdFromHost() || localStorage.getItem(storedFleetIdKey) || '';
		if (fleetIdInput) localStorage.setItem(storedFleetIdKey, fleetIdInput);
		loadCompanionReleases();
		loadAdminSession();
		loadUsers();
		checkDevice();
		loadBotProfile();
		loadCredentials();
		loadWorkspaceSettings();
		loadAttendanceLocations();
	});

	function activateAdminSection(section: AdminSection) {
		activeAdminSection = section;
	}

	function fleetIdFromHost() {
		if (!browser) return '';
		const host = location.hostname;
		if (isLocalBrowserHost()) return '';
		const suffix = '.intern.kim';
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
		loadCredentials();
		loadWorkspaceSettings();
		loadAttendanceLocations();
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
				botProfileErrorMessage = text.messages.botProfileLoadError;
				return;
			}
			applyBotProfile((await response.json()) as BotProfile);
		} catch {
			botProfileErrorMessage = text.messages.botProfileLoadError;
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
			botProfileErrorMessage = text.messages.botProfileSaveError;
		} finally {
			isSavingBotProfile = false;
		}
	}

	async function loadCredentials() {
		if (!adminBaseURL()) return;

		isLoadingCredentials = true;
		credentialErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/credentials/providers`, { credentials: 'include' });
			if (!response.ok) {
				credentialErrorMessage = text.messages.credentialsLoadError;
				return;
			}
			const data = (await response.json()) as CredentialProvidersResponse;
			credentialProviders = data.providers ?? [];
		} catch {
			credentialErrorMessage = text.messages.credentialsLoadError;
		} finally {
			isLoadingCredentials = false;
		}
	}

	function openRouterProvider() {
		return credentialProviders.find((provider) => provider.provider === 'openrouter');
	}

	async function saveOpenRouterKey() {
		if (!adminBaseURL() || !openRouterApiKey.trim()) return;

		isSavingCredential = true;
		credentialErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/credentials/openrouter-key`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ apiKey: openRouterApiKey.trim() })
			});
			if (!response.ok) {
				credentialErrorMessage = (await response.text()).trim() || text.messages.openRouterSaveError;
				return;
			}
			const provider = (await response.json()) as CredentialProviderStatus;
			credentialProviders = [provider, ...credentialProviders.filter((candidate) => candidate.provider !== provider.provider)];
			openRouterApiKey = '';
		} catch {
			credentialErrorMessage = text.messages.openRouterSaveError;
		} finally {
			isSavingCredential = false;
		}
	}

	async function deleteOpenRouterKey() {
		if (!adminBaseURL()) return;

		isSavingCredential = true;
		credentialErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/credentials/openrouter-key`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) {
				credentialErrorMessage = (await response.text()).trim() || text.messages.openRouterDeleteError;
				return;
			}
			const provider = (await response.json()) as CredentialProviderStatus;
			credentialProviders = [provider, ...credentialProviders.filter((candidate) => candidate.provider !== provider.provider)];
		} catch {
			credentialErrorMessage = text.messages.openRouterDeleteError;
		} finally {
			isSavingCredential = false;
		}
	}

	async function loadWorkspaceSettings() {
		if (!adminBaseURL()) return;

		isLoadingWorkspaceSettings = true;
		workspaceSettingsMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/workspace-settings`, { credentials: 'include' });
			if (!response.ok) {
				workspaceSettingsMessage = text.settings.loadError;
				return;
			}
			workspaceSettings = normalizeWorkspaceSettings((await response.json()) as WorkspaceSettings);
			workspaceSettingsDraft = { ...workspaceSettings };
		} catch {
			workspaceSettingsMessage = text.settings.loadError;
		} finally {
			isLoadingWorkspaceSettings = false;
		}
	}

	async function saveWorkspaceSettings() {
		if (!adminBaseURL()) return;

		isSavingWorkspaceSettings = true;
		workspaceSettingsMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/workspace-settings`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(workspaceSettingsDraft)
			});
			if (!response.ok) {
				workspaceSettingsMessage = await response.text();
				return;
			}
			workspaceSettings = normalizeWorkspaceSettings((await response.json()) as WorkspaceSettings);
			workspaceSettingsDraft = { ...workspaceSettings };
			workspaceSettingsMessage = text.settings.saveSuccess;
		} catch {
			workspaceSettingsMessage = text.settings.saveError;
		} finally {
			isSavingWorkspaceSettings = false;
		}
	}

	function normalizeWorkspaceSettings(settings: WorkspaceSettings): WorkspaceSettings {
		return {
			timeZone: settings.timeZone?.trim() || 'system',
			language: settings.language === 'en' ? 'en' : 'ko',
			updatedAt: settings.updatedAt
		};
	}

	async function loadAttendanceLocations() {
		if (!adminBaseURL()) return;

		isLoadingAttendanceLocations = true;
		attendanceLocationsMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/attendance-locations`, { credentials: 'include' });
			if (!response.ok) {
				attendanceLocationsMessage = text.attendanceLocations.loadError;
				return;
			}
			const data = (await response.json()) as AttendanceLocationsResponse;
			attendanceLocations = data.locations ?? [];
		} catch {
			attendanceLocationsMessage = text.attendanceLocations.loadError;
		} finally {
			isLoadingAttendanceLocations = false;
		}
	}

	async function saveAttendanceLocations() {
		if (!adminBaseURL()) return;

		isSavingAttendanceLocations = true;
		attendanceLocationsMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/attendance-locations`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ locations: attendanceLocations })
			});
			if (!response.ok) {
				attendanceLocationsMessage = await response.text();
				return;
			}
			const data = (await response.json()) as AttendanceLocationsResponse;
			attendanceLocations = data.locations ?? [];
			attendanceLocationsMessage = text.attendanceLocations.saveSuccess;
		} catch {
			attendanceLocationsMessage = text.attendanceLocations.saveError;
		} finally {
			isSavingAttendanceLocations = false;
		}
	}

	function addAttendanceLocation() {
		attendanceLocations = [
			...attendanceLocations,
			{
				id: '',
				name: '',
				color: '#0ea5e9',
				isDefault: attendanceLocations.length === 0
			}
		];
	}

	function removeAttendanceLocation(index: number) {
		if (attendanceLocations.length <= 1) return;
		const removedLocation = attendanceLocations[index];
		const nextLocations = attendanceLocations.filter((_, locationIndex) => locationIndex !== index);
		if (removedLocation.isDefault && nextLocations[0]) {
			nextLocations[0] = { ...nextLocations[0], isDefault: true };
		}
		attendanceLocations = nextLocations;
	}

	function updateAttendanceLocation(index: number, field: keyof AttendanceLocation, value: string | boolean) {
		attendanceLocations = attendanceLocations.map((location, locationIndex) => {
			if (locationIndex !== index) return field === 'isDefault' ? { ...location, isDefault: false } : location;
			return { ...location, [field]: value };
		});
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
			const response = await fetch(`${usersBaseURL()}/users?includePolicy=true`, { credentials: 'include' });
			if (!response.ok) {
				errorMessage =
					response.status === 403
						? text.messages.adminAuthRequiredOnDevice
						: text.messages.usersLoadError;
				return;
			}
			const data = (await response.json()) as UsersResponse;
			applyUsersResponse(data);
		} catch {
			errorMessage = text.messages.usersLoadError;
		} finally {
			isLoadingUsers = false;
		}
	}

	function applyUsersResponse(data: UsersResponse) {
		availableCircles = data.availableCircles?.length ? data.availableCircles : [{ circleID: 'staff', displayName: 'Staff' }];
		if (data.records) {
			userRecords = data.records.map((record) => ({
				...record,
				name: record.name ?? '',
				hireDate: record.hireDate ?? '',
				circles: normalizeUserCircles(record.circles, record.role)
			}));
		} else {
			userRecords = (data.users ?? []).map((email, index) => ({
				userID: `legacy-${index}`,
				handle: email.split('@')[0]?.toLowerCase() ?? '',
				email,
				hireDate: '',
				role: index === 0 ? 'admin' : 'member',
				circles: index === 0 ? ['staff', 'admin'] : ['staff']
			}));
		}
		if (data.temporaryPassword && data.temporaryPasswordEmail) {
			temporaryPasswordResult = {
				email: data.temporaryPasswordEmail,
				password: data.temporaryPassword
			};
		}
	}

	function normalizeUserCircles(circles: string[] | undefined, role: UserRole) {
		const result = new Set(['staff', ...(circles ?? []).map((circle) => circle.trim().toLowerCase()).filter(Boolean)]);
		if (role === 'admin') result.add('admin');
		return [...result];
	}

	function hasUserCircle(record: UserRecord, circleID: string) {
		return normalizeUserCircles(record.circles, record.role).includes(circleID);
	}

	function toggleUserCircle(record: UserRecord, circleID: string) {
		if (circleID === 'staff') return;
		const current = new Set(normalizeUserCircles(record.circles, record.role));
		if (current.has(circleID)) current.delete(circleID);
		else current.add(circleID);
		record.circles = normalizeUserCircles([...current], record.role);
	}

	async function saveCircle() {
		if (!adminBaseURL() || !newCircleID.trim()) return;
		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/circles`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					circleID: newCircleID.trim().toLowerCase(),
					displayName: newCircleName.trim() || newCircleID.trim(),
					isMattermostManaged: newCircleMattermostManaged
				})
			});
			if (!response.ok) {
				errorMessage = (await response.text()).trim() || text.messages.userSaveError;
				return;
			}
			newCircleID = '';
			newCircleName = '';
			newCircleMattermostManaged = true;
			await loadUsers();
		} catch {
			errorMessage = text.messages.userSaveError;
		} finally {
			isSavingUser = false;
		}
	}

	async function deleteCircle(circleID: string) {
		if (!adminBaseURL() || circleID === 'staff') return;
		isSavingUser = true;
		errorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/circles/${encodeURIComponent(circleID)}`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) {
				errorMessage = (await response.text()).trim() || text.messages.userRemoveError;
				return;
			}
			await loadUsers();
		} catch {
			errorMessage = text.messages.userRemoveError;
		} finally {
			isSavingUser = false;
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
			const response = await fetch(`${usersBaseURL()}/users?includePolicy=true`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ handle, name, email, hireDate: newHireDate, role: newUserRole })
			});
			if (!response.ok) {
				const detail = (await response.text()).trim();
				errorMessage = response.status === 403
					? text.messages.adminAuthRequiredOnDevice
					: detail || text.messages.userInviteError;
				return;
			}
			const data = (await response.json()) as UsersResponse;
			applyUsersResponse(data);
			await loadAdminSession();
			newHandle = '';
			newName = '';
			newEmail = '';
			newHireDate = '';
			newUserRole = 'member';
		} catch {
			errorMessage = text.messages.userInviteError;
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
			const response = await fetch(`${usersBaseURL()}/users?includePolicy=true`, {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					userID: record.userID,
					handle: normalizeHandle(record.handle),
					name: record.name?.trim() ?? '',
					hireDate: record.hireDate ?? '',
					email: record.email,
					role,
					circles: normalizeUserCircles(record.circles, role),
					mattermostUserID: record.mattermostUserID,
					mattermostUsername: record.mattermostUsername,
					status: record.status
				})
			});
			if (!response.ok) {
				const detail = (await response.text()).trim();
				errorMessage = response.status === 403 ? text.messages.adminAuthRequired : detail || text.messages.userSaveError;
				return;
			}
			applyUsersResponse((await response.json()) as UsersResponse);
		} catch {
			errorMessage = text.messages.userSaveError;
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
			const response = await fetch(`${usersBaseURL()}/users/${encodeURIComponent(email)}?includePolicy=true`, {
				method: 'DELETE',
				credentials: 'include'
			});
			if (!response.ok) {
				errorMessage =
					response.status === 403
						? text.messages.adminAuthRequiredOnDevice
						: text.messages.userRemoveError;
				return;
			}
			const data = (await response.json()) as UsersResponse;
			applyUsersResponse(data);
		} catch {
			errorMessage = text.messages.userRemoveError;
		} finally {
			isSavingUser = false;
		}
	}

	async function resetUserPassword(record: UserRecord) {
		if (!fleetId()) return;
		if (!confirm(text.users.resetPasswordConfirm)) return;

		isSavingUser = true;
		errorMessage = '';
		temporaryPasswordResult = null;
		try {
			const response = await fetch(`${usersBaseURL()}/users/${encodeURIComponent(record.email)}/password-reset`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!response.ok) {
				errorMessage = (await response.text()).trim() || text.users.resetPasswordError;
				return;
			}
			const data = (await response.json()) as UsersResponse;
			if (data.temporaryPassword && data.temporaryPasswordEmail) {
				temporaryPasswordResult = {
					email: data.temporaryPasswordEmail,
					password: data.temporaryPassword
				};
			}
		} catch {
			errorMessage = text.users.resetPasswordError;
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
			if (!response.ok) adminErrorMessage = text.messages.deviceUnreachable;
		} catch {
			isDeviceReachable = false;
			adminErrorMessage = text.messages.deviceUnreachable;
		} finally {
			isCheckingDevice = false;
		}
		if (!isDeviceReachable) {
			blueclawUpdateStatus = null;
			blueclawUpdateJob = null;
		}
		if (isDeviceReachable) await loadCompanions();
		if (isDeviceReachable) await loadWorkspaceSettings();
		if (isDeviceReachable) await loadBlueclawUpdateStatus();
	}

	async function loadBlueclawUpdateStatus() {
		if (!adminBaseURL()) return;

		isLoadingBlueclawUpdate = true;
		blueclawUpdateMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/updates/status`, { credentials: 'include' });
			if (!response.ok) {
				blueclawUpdateMessage = text.deviceUpdate.loadError;
				return;
			}
			blueclawUpdateStatus = (await response.json()) as BlueclawUpdateStatus;
			blueclawUpdateJob = blueclawUpdateStatus.activeJob ?? blueclawUpdateJob;
		} catch {
			blueclawUpdateMessage = text.deviceUpdate.loadError;
		} finally {
			isLoadingBlueclawUpdate = false;
		}
	}

	async function applyBlueclawUpdate() {
		if (!adminBaseURL()) return;

		isApplyingBlueclawUpdate = true;
		blueclawUpdateMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/updates/apply`, {
				method: 'POST',
				credentials: 'include'
			});
			if (!response.ok) {
				blueclawUpdateMessage = (await response.text()).trim() || text.deviceUpdate.applyError;
				return;
			}
			blueclawUpdateJob = (await response.json()) as AdminJob;
			await pollBlueclawUpdateJob(blueclawUpdateJob.jobID);
		} catch {
			blueclawUpdateMessage = text.deviceUpdate.applyError;
		} finally {
			isApplyingBlueclawUpdate = false;
		}
	}

	async function pollBlueclawUpdateJob(jobID: string) {
		for (let attempt = 0; attempt < 120; attempt += 1) {
			const response = await fetch(`${adminBaseURL()}/updates/jobs/${encodeURIComponent(jobID)}`, { credentials: 'include' });
			if (response.ok) {
				blueclawUpdateJob = (await response.json()) as AdminJob;
				if (blueclawUpdateJob.status === 'completed' || blueclawUpdateJob.status === 'failed' || blueclawUpdateJob.status === 'already_current') {
					await loadBlueclawUpdateStatus();
					return;
				}
			}
			await new Promise((resolve) => setTimeout(resolve, 1500));
		}
	}

	async function loadCompanions() {
		if (!adminBaseURL()) return;

		isLoadingCompanions = true;
		companionErrorMessage = '';
		try {
			const response = await fetch(`${adminBaseURL()}/companion/status`, { credentials: 'include' });
			if (!response.ok) {
				companionErrorMessage = text.messages.companionStatusError;
				return;
			}
			const data = (await response.json()) as CompanionStatusResponse;
			companionStatuses = data.companions ?? [];
		} catch {
			companionErrorMessage = text.messages.companionStatusError;
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
				companionErrorMessage = text.messages.companionPairingError;
				return;
			}
			companionPairingCode = (await response.json()) as CompanionPairingCodeResponse;
			if (companionPairingCode.deepLink && browser) location.href = companionPairingCode.deepLink;
		} catch {
			companionErrorMessage = text.messages.companionPairingError;
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
				companionErrorMessage = text.messages.companionRevokeError;
				return;
			}
			await loadCompanions();
		} catch {
			companionErrorMessage = text.messages.companionRevokeError;
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
				adminErrorMessage = text.messages.backupStartError;
				return;
			}
			backupJob = (await response.json()) as AdminJob;
			await pollJob('backup', backupJob.jobID);
		} catch {
			adminErrorMessage = text.messages.backupStartError;
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
				adminErrorMessage = text.messages.restoreStartError;
				return;
			}
			const upload = (await uploadResponse.json()) as RestoreUploadResponse;
			const chunkCount = Math.ceil(restoreBundle.size / upload.chunkSize);
			for (let chunkIndex = 0; chunkIndex < chunkCount; chunkIndex += 1) {
				const start = chunkIndex * upload.chunkSize;
				const end = Math.min(start + upload.chunkSize, restoreBundle.size);
				adminErrorMessage = `${text.messages.restoreUploadProgress} ${chunkIndex + 1}/${chunkCount}`;
				const chunkResponse = await fetch(`${adminBaseURL()}/restore/uploads/${upload.uploadID}/chunks/${chunkIndex}`, {
					method: 'PUT',
					credentials: 'include',
					body: restoreBundle.slice(start, end)
				});
				if (!chunkResponse.ok) {
					adminErrorMessage = text.messages.restoreUploadError;
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
				adminErrorMessage = text.messages.restoreStartError;
				return;
			}
			restoreJob = (await completeResponse.json()) as AdminJob;
			await pollJob('restore', restoreJob.jobID);
		} catch {
			adminErrorMessage = text.messages.restoreStartError;
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

<main class="bg-background text-foreground min-h-svh w-full min-w-0 flex-1">
	<div class="mx-auto flex min-h-svh w-full max-w-6xl min-w-0 flex-col px-4 py-5 sm:px-5 sm:py-6">
		<header class="flex flex-wrap items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<img src={logoSrc} alt={text.title} class="size-9" />
				<div>
					<h1 class="text-lg font-semibold leading-tight">{text.title}</h1>
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
						<Button href={mattermostURL()} data-sveltekit-reload class="gap-2">
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

		<nav class="flex w-full min-w-0 gap-1 overflow-x-auto py-4">
			{#each adminSections() as section}
				<Button
					variant={activeAdminSection === section.value ? 'default' : 'ghost'}
					size="sm"
					onclick={() => activateAdminSection(section.value)}
				>
					{section.label}
				</Button>
			{/each}
		</nav>

		<section class="grid min-w-0 gap-5 py-6">
			{#if activeAdminSection === 'device'}
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
							<p class="mt-1 font-mono text-sm">{shortRevision(blueclawUpdateStatus?.current?.releaseID)}</p>
						</div>
						<div class="rounded-md bg-muted/30 p-3">
							<p class="text-muted-foreground text-xs">{text.deviceUpdate.latestRevision}</p>
							<p class="mt-1 font-mono text-sm">{shortRevision(blueclawUpdateStatus?.latest?.releaseID)}</p>
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
								onclick={applyBlueclawUpdate}
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
					<Input bind:value={botProfile.displayName} placeholder={text.bot.displayNamePlaceholder} disabled={isLoadingBotProfile} />
					<Input bind:value={botProfile.englishDisplayName} placeholder={text.bot.englishDisplayNamePlaceholder} disabled={isLoadingBotProfile} />
					<Input
						class="md:col-span-2"
						bind:value={botProfile.publicDescription}
						placeholder={text.bot.publicDescriptionPlaceholder}
						disabled={isLoadingBotProfile}
					/>
					<Textarea
						bind:value={botProfileAliasesText}
						placeholder={text.bot.aliasesPlaceholder}
						disabled={isLoadingBotProfile}
						class="min-h-24"
					/>
					<Textarea
						bind:value={botProfile.identityExtension}
						placeholder={text.bot.identityExtensionPlaceholder}
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

			{#if activeAdminSection === 'credentials'}
				<div class="rounded-lg border p-4">
				<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
					<div>
						<h3 class="text-sm font-semibold">{text.credentials.title}</h3>
						<p class="text-muted-foreground mt-1 text-sm">
							{text.credentials.description}
						</p>
					</div>
					<Badge variant={openRouterProvider()?.configured ? 'secondary' : 'outline'}>
						{openRouterProvider()?.configured ? text.credentials.configured : text.credentials.missing}
					</Badge>
				</div>
				<div class="grid gap-3">
					<div class="rounded-md bg-muted/30 p-3">
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<p class="text-sm font-medium">OpenRouter</p>
								<p class="text-muted-foreground mt-1 text-xs">
									{#if openRouterProvider()?.fingerprint}
										{openRouterProvider()?.fingerprint}
									{:else if isLoadingCredentials}
										{text.credentials.loading}
									{:else}
										{text.credentials.noKey}
									{/if}
								</p>
							</div>
							{#if openRouterProvider()?.configured}
								<Button variant="ghost" size="sm" disabled={isSavingCredential} onclick={deleteOpenRouterKey}>
									{text.credentials.delete}
								</Button>
							{/if}
						</div>
					</div>
					<form
						class="grid gap-2 sm:grid-cols-[1fr_auto]"
						onsubmit={(event) => {
							event.preventDefault();
							saveOpenRouterKey();
						}}
					>
						<Input bind:value={openRouterApiKey} type="password" placeholder={text.credentials.openRouterApiKeyPlaceholder} autocomplete="new-password" />
						<Button type="submit" disabled={!isDeviceReachable || isSavingCredential || !openRouterApiKey.trim()} class="gap-2">
							{#if isSavingCredential}
								<LoaderIcon class="size-4 animate-spin" />
							{/if}
							{text.credentials.save}
						</Button>
					</form>
					<p class="text-muted-foreground text-xs">{text.credentials.notice}</p>
				</div>
				{#if credentialErrorMessage}
					<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{credentialErrorMessage}
					</p>
				{/if}
			</div>
			{/if}

			{#if activeAdminSection === 'settings'}
				<div class="rounded-lg border p-4">
					<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
						<div>
							<h3 class="text-sm font-semibold">{text.settings.title}</h3>
							<p class="text-muted-foreground mt-1 text-sm">{text.settings.description}</p>
						</div>
						<div class="flex flex-wrap gap-2">
							<Badge variant="outline">{workspaceSettings.timeZone || 'system'}</Badge>
							<Badge variant="outline">{workspaceLanguageOptions().find((option) => option.value === workspaceSettings.language)?.label}</Badge>
						</div>
					</div>
					<div class="grid gap-3 md:grid-cols-[1fr_220px_auto] md:items-end">
						<div>
							<label class="text-xs font-medium text-muted-foreground" for="workspace-time-zone">{text.settings.timeZone}</label>
							<Input
								id="workspace-time-zone"
								bind:value={workspaceSettingsDraft.timeZone}
								placeholder={text.settings.timeZonePlaceholder}
								disabled={isLoadingWorkspaceSettings}
								autocomplete="off"
								class="mt-1"
							/>
							<p class="mt-2 text-xs text-muted-foreground">{text.settings.timeZoneHint}</p>
						</div>
						<label class="grid gap-1.5">
							<span class="text-xs font-medium text-muted-foreground">{text.settings.workspaceLanguageTitle}</span>
							<Select.Root type="single" bind:value={workspaceSettingsDraft.language} disabled={isLoadingWorkspaceSettings || isSavingWorkspaceSettings}>
								<Select.Trigger class="w-full">
									{workspaceLanguageOptions().find((option) => option.value === workspaceSettingsDraft.language)?.label}
								</Select.Trigger>
								<Select.Content>
									{#each workspaceLanguageOptions() as option (option.value)}
										<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
							<span class="text-xs text-muted-foreground">{text.settings.workspaceLanguageDescription}</span>
						</label>
						<Button disabled={!isDeviceReachable || isLoadingWorkspaceSettings || isSavingWorkspaceSettings} onclick={saveWorkspaceSettings}>
							{#if isSavingWorkspaceSettings}
								<LoaderIcon class="size-4 animate-spin" />
							{/if}
							{text.settings.save}
						</Button>
					</div>
					{#if workspaceSettingsMessage}
						<p class="mt-3 rounded-md border bg-muted/30 px-3 py-2 text-sm">{workspaceSettingsMessage}</p>
					{/if}
				</div>

				<div class="rounded-lg border p-4">
					<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
						<div>
							<h3 class="flex items-center gap-2 text-sm font-semibold">
								<MapPinIcon class="size-4 text-emerald-600" />
								{text.attendanceLocations.title}
							</h3>
							<p class="mt-1 text-sm text-muted-foreground">{text.attendanceLocations.description}</p>
						</div>
						<Button variant="outline" size="sm" class="gap-2" onclick={addAttendanceLocation} disabled={isLoadingAttendanceLocations}>
							<PlusIcon class="size-4" />
							{text.attendanceLocations.add}
						</Button>
					</div>
					<div class="grid gap-2">
						{#each attendanceLocations as location, index (index)}
							<div class="grid gap-2 rounded-md border p-3 md:grid-cols-[auto_1fr_9rem_auto_auto] md:items-center">
								<input
									type="color"
									value={location.color}
									aria-label={text.attendanceLocations.color}
									class="size-9 rounded-md border bg-background"
									oninput={(event) => updateAttendanceLocation(index, 'color', event.currentTarget.value)}
								/>
								<Input
									value={location.name}
									placeholder={text.attendanceLocations.placeholder}
									autocomplete="off"
									oninput={(event) => updateAttendanceLocation(index, 'name', event.currentTarget.value)}
								/>
								<Button
									variant={location.isDefault ? 'secondary' : 'ghost'}
									size="sm"
									onclick={() => updateAttendanceLocation(index, 'isDefault', true)}
								>
									{text.attendanceLocations.default}
								</Button>
								{#if attendanceLocations.length > 1}
									<Button variant="ghost" size="icon-sm" aria-label={text.attendanceLocations.remove} onclick={() => removeAttendanceLocation(index)}>
										<Trash2Icon class="size-4" />
									</Button>
								{/if}
							</div>
						{/each}
					</div>
					<div class="mt-4 flex flex-wrap items-center gap-2">
						<Button disabled={!isDeviceReachable || isSavingAttendanceLocations || attendanceLocations.length === 0} onclick={saveAttendanceLocations}>
							{#if isSavingAttendanceLocations}
								<LoaderIcon class="size-4 animate-spin" />
							{/if}
							{text.attendanceLocations.save}
						</Button>
						{#if attendanceLocationsMessage}
							<p class="rounded-md border bg-muted/30 px-3 py-2 text-sm">{attendanceLocationsMessage}</p>
						{/if}
					</div>
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
					<Badge variant={onlineCompanionCount() > 0 ? 'secondary' : 'outline'}>{onlineCompanionCount()} {text.companion.online}</Badge>
				</div>

				<div class="grid gap-4 lg:grid-cols-[1fr_1fr]">
					<div class="grid gap-3 rounded-md bg-muted/30 p-3">
						{#if recommendedCompanionRelease()}
							<div>
								<p class="text-sm font-medium">{recommendedCompanionRelease()?.label} {text.companion.companionSuffix}</p>
								<p class="text-muted-foreground text-xs">
									{recommendedCompanionRelease()?.architecture}
									{#if !isCompanionReleaseAvailable(recommendedCompanionRelease())}
										· {text.companion.comingSoon}
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
										<Button disabled variant="ghost" size="sm">{release.label} {text.companion.comingSoon}</Button>
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
									{text.companion.expires} {new Date(companionPairingCode.expiresAt).toLocaleTimeString()}
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
										<Badge variant={companion.isOnline ? 'secondary' : 'outline'}>{companion.isOnline ? text.companion.online : text.companion.offline}</Badge>
										{#if companion.localOnly}
											<Badge variant="outline">{text.companion.localOnly}</Badge>
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
						<Input bind:value={backupPassphrase} type="password" placeholder={text.backup.passphrasePlaceholder} autocomplete="new-password" />
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
										{backupJob.manifest.fleetID || fleetId()} · {backupJob.manifest.components?.join(', ') || text.backup.manifestReady}
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
			{/if}

		</section>

		{#if activeAdminSection === 'users'}
			<Separator />

			<section class="grid min-w-0 gap-5 py-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h2 class="text-base font-semibold">{text.users.title}</h2>
					<p class="text-muted-foreground text-sm">
						{text.users.description}
					</p>
				</div>
				<Badge variant="outline">{userCount()} {text.users.userCount}</Badge>
			</div>

			{#if !isDeviceContext()}
				<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">
					{text.users.deviceOnly}
				</p>
			{:else}
				<Card.Root>
					<Card.Header class="gap-1">
						<Card.Title class="text-sm">{text.users.inviteTitle}</Card.Title>
						<Card.Description>{text.users.inviteDescription}</Card.Description>
					</Card.Header>
					<Card.Content>
						<form
							class="grid gap-3 lg:grid-cols-[minmax(120px,0.8fr)_minmax(150px,1fr)_minmax(210px,1.3fr)_150px_120px_auto] lg:items-end"
							onsubmit={(event) => {
								event.preventDefault();
								addEmail();
							}}
						>
							<label class="grid gap-1.5">
								<Label>{text.users.handle}</Label>
								<Input bind:value={newHandle} placeholder="chanhee" autocomplete="off" />
							</label>
							<label class="grid gap-1.5">
								<Label>{text.users.realName}</Label>
								<Input bind:value={newName} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
							</label>
							<label class="grid gap-1.5">
								<Label>{text.users.email}</Label>
								<div class="relative">
									<MailIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
									<Input bind:value={newEmail} type="email" placeholder={text.users.emailPlaceholder} class="pl-9" />
								</div>
							</label>
							<label class="grid gap-1.5">
								<Label>{text.users.hireDate}</Label>
								<Input bind:value={newHireDate} type="date" />
							</label>
							<label class="grid gap-1.5">
								<Label>{text.users.role}</Label>
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
							</label>
							<Button type="submit" disabled={isSavingUser || !newEmail.trim() || !newName.trim() || !isValidHandle(newHandle)} class="gap-2">
								{#if isSavingUser}
									<LoaderIcon class="size-4 animate-spin" />
								{:else}
									<PlusIcon class="size-4" />
								{/if}
								{text.users.invite}
							</Button>
						</form>
					</Card.Content>
				</Card.Root>

				<p class="text-muted-foreground text-sm">
					{text.users.passwordNotice}
				</p>

				<Card.Root>
					<Card.Header class="gap-1">
						<Card.Title class="text-sm">{text.users.groupTitle}</Card.Title>
						<Card.Description>{text.users.groupDescription}</Card.Description>
					</Card.Header>
					<Card.Content>
						<form
							class="grid gap-3 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end"
							onsubmit={(event) => {
								event.preventDefault();
								saveCircle();
							}}
						>
							<label class="grid gap-1.5">
								<Label>{text.users.groupID}</Label>
								<Input bind:value={newCircleID} placeholder={text.users.groupIDPlaceholder} autocomplete="off" />
							</label>
							<label class="grid gap-1.5">
								<Label>{text.users.groupName}</Label>
								<Input bind:value={newCircleName} placeholder={text.users.groupNamePlaceholder} autocomplete="off" />
							</label>
							<label class="flex items-center gap-2 text-sm">
								<input type="checkbox" bind:checked={newCircleMattermostManaged} />
								{text.users.mattermostManaged}
							</label>
							<Button type="submit" disabled={isSavingUser || !newCircleID.trim()}>{text.users.addGroup}</Button>
						</form>
						<div class="mt-3 flex flex-wrap gap-2">
							{#each availableCircles as circle (circle.circleID)}
								<Badge variant="outline" class="gap-2">
									{circle.displayName || circle.circleID}
									{#if circle.circleID !== 'staff'}
										<button type="button" class="text-muted-foreground hover:text-destructive" onclick={() => deleteCircle(circle.circleID)}>
											<XIcon class="size-3" />
										</button>
									{/if}
								</Badge>
							{/each}
						</div>
					</Card.Content>
				</Card.Root>

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
					<Card.Root class="overflow-hidden">
						<Card.Header class="flex-row items-center justify-between gap-3 border-b">
							<div>
								<Card.Title class="text-sm">{text.users.directoryTitle}</Card.Title>
								<Card.Description>{text.users.directoryDescription}</Card.Description>
							</div>
							<Badge variant="secondary">{adminCount()} {text.users.adminCount}</Badge>
						</Card.Header>
						<Card.Content class="p-0">
							<div class="hidden overflow-x-auto lg:block">
								<Table.Root class="min-w-[1260px]">
									<Table.Header class="bg-muted/40">
										<Table.Row class="hover:bg-transparent">
											<Table.Head class="w-[250px]">{text.users.person}</Table.Head>
											<Table.Head class="w-[160px]">{text.users.handle}</Table.Head>
											<Table.Head class="w-[180px]">{text.users.realName}</Table.Head>
											<Table.Head class="w-[150px]">{text.users.hireDate}</Table.Head>
											<Table.Head class="w-[110px]">{text.users.role}</Table.Head>
											<Table.Head class="w-[220px]">{text.users.groups}</Table.Head>
											<Table.Head class="text-right">{text.users.actions}</Table.Head>
										</Table.Row>
									</Table.Header>
									<Table.Body>
										{#each userRecordsByHireDate() as record (record.email)}
											<Table.Row>
												<Table.Cell>
													<div class="flex min-w-0 items-center gap-3">
														<PersonAvatar name={record.name} email={record.email} class="size-9" />
														<div class="min-w-0">
															<p class="truncate text-sm font-medium">{record.email}</p>
															<p class="truncate text-xs text-muted-foreground">
																{record.mattermostUsername ? `Mattermost: ${record.mattermostUsername}` : text.users.noMattermost}
															</p>
															{#if record.isIncomplete}
																<p class="text-xs text-destructive">{text.users.incomplete}</p>
															{/if}
														</div>
													</div>
												</Table.Cell>
												<Table.Cell>
													<Input bind:value={record.handle} placeholder={text.users.handlePlaceholder} autocomplete="off" />
												</Table.Cell>
												<Table.Cell>
													<Input bind:value={record.name} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
												</Table.Cell>
												<Table.Cell>
													<Input bind:value={record.hireDate} type="date" />
												</Table.Cell>
												<Table.Cell>
													<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{record.role}</Badge>
												</Table.Cell>
												<Table.Cell>
													<div class="flex flex-wrap gap-1.5">
														{#each visibleCircles() as circle (circle.circleID)}
															<Button
																type="button"
																variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
																size="sm"
																disabled={circle.circleID === 'staff' || isSavingUser}
																onclick={() => toggleUserCircle(record, circle.circleID)}
																title={circle.isMattermostManaged ? text.users.mattermostManaged : ''}
															>
																{circle.displayName || circle.circleID}
															</Button>
														{/each}
													</div>
												</Table.Cell>
												<Table.Cell>
													{@render UserActions(record, 'desktop')}
												</Table.Cell>
											</Table.Row>
										{/each}
									</Table.Body>
								</Table.Root>
							</div>
							<div class="grid gap-0 lg:hidden">
								{#each userRecordsByHireDate() as record (record.email)}
									<div class="grid gap-3 border-b p-4 last:border-b-0">
										<div class="flex min-w-0 items-start justify-between gap-3">
											<div class="flex min-w-0 items-center gap-3">
												<PersonAvatar name={record.name} email={record.email} class="size-10" />
												<div class="min-w-0">
													<p class="truncate text-sm font-medium">{record.name || record.email}</p>
													<p class="truncate text-xs text-muted-foreground">{record.email}</p>
													{#if record.mattermostUsername}
														<p class="truncate text-xs text-muted-foreground">Mattermost: {record.mattermostUsername}</p>
													{/if}
												</div>
											</div>
											<Badge variant={record.role === 'admin' ? 'secondary' : 'outline'}>{record.role}</Badge>
										</div>
										<div class="grid gap-3 sm:grid-cols-3">
											<label class="grid gap-1.5">
												<Label>{text.users.handle}</Label>
												<Input bind:value={record.handle} placeholder={text.users.handlePlaceholder} autocomplete="off" />
											</label>
											<label class="grid gap-1.5">
												<Label>{text.users.realName}</Label>
												<Input bind:value={record.name} placeholder={text.users.realNamePlaceholder} autocomplete="off" />
											</label>
											<label class="grid gap-1.5">
												<Label>{text.users.hireDate}</Label>
												<Input bind:value={record.hireDate} type="date" />
											</label>
										</div>
										<div class="flex flex-wrap gap-1.5">
											{#each visibleCircles() as circle (circle.circleID)}
												<Button
													type="button"
													variant={hasUserCircle(record, circle.circleID) ? 'secondary' : 'outline'}
													size="sm"
													disabled={circle.circleID === 'staff' || isSavingUser}
													onclick={() => toggleUserCircle(record, circle.circleID)}
												>
													{circle.displayName || circle.circleID}
												</Button>
											{/each}
										</div>
										{#if record.isIncomplete}
											<p class="rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">{text.users.incomplete}</p>
										{/if}
										{@render UserActions(record, 'mobile')}
									</div>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>
				{/if}
			{/if}
			</section>
		{/if}
	</div>
</main>

{#snippet UserActions(record: UserRecord, layout: 'desktop' | 'mobile')}
	<div class={layout === 'desktop' ? 'flex justify-end gap-2' : 'grid gap-2 sm:grid-cols-4'}>
		<Button variant="outline" size="sm" disabled={isSavingUser || !isValidUserRecord(record)} onclick={() => saveUser(record)}>
			{text.users.save}
		</Button>
		<Button
			class="gap-2"
			variant="outline"
			size="sm"
			disabled={isSavingUser || !isValidUserRecord(record)}
			onclick={() => resetUserPassword(record)}
		>
			<RefreshCwIcon class="size-4" />
			{text.users.resetPassword}
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
			size={layout === 'desktop' ? 'icon-sm' : 'sm'}
			disabled={isSavingUser || (record.role === 'admin' && adminCount() <= 1)}
			onclick={() => removeEmail(record.email)}
			aria-label={text.users.remove}
		>
			<XIcon class="size-4" />
			{#if layout === 'mobile'}
				<span>{text.users.remove}</span>
			{/if}
		</Button>
	</div>
{/snippet}
