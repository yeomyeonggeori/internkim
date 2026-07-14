<script lang="ts">
	import { browser } from '$app/environment';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import ShieldCheckIcon from '@lucide/svelte/icons/shield-check';
	import QrCode from 'svelte-qrcode';
	import { onMount } from 'svelte';
	import { fetchAdminSession } from './admin/admin-api';
	import { adminSessionRole, canViewAdminSection, firstVisibleAdminSection } from './admin/admin-role-policy';
	import BackupSection from './admin/backup-section.svelte';
	import BotSection from './admin/bot-section.svelte';
	import CredentialsSection from './admin/credentials-section.svelte';
	import CompanyShareSection from './admin/company-share-section.svelte';
	import DeviceSection from './admin/device-section.svelte';
	import NetworkSection from './admin/network-section.svelte';
	import SettingsSection from './admin/settings-section.svelte';
	import type { AdminSection, AdminSession } from './admin/admin-types';
	import { adminText } from './admin/text';
	import UsersSection from './admin/users-section.svelte';

	const logoSrc = '/logo.svg';
	const storedFleetIdKey = 'internkim_fleet_id';
	const isMockAdminAPI = import.meta.env.VITE_MOCK_ADMIN === '1';
	const text = createPageText(adminText);
	const adminSectionConfigurations: { value: AdminSection; isDeviceManagedOnly: boolean }[] = [
		{ value: 'device', isDeviceManagedOnly: true },
		{ value: 'users', isDeviceManagedOnly: false },
		{ value: 'credentials', isDeviceManagedOnly: false },
		{ value: 'backup', isDeviceManagedOnly: false },
		{ value: 'bot', isDeviceManagedOnly: false },
		{ value: 'settings', isDeviceManagedOnly: false },
		{ value: 'sharing', isDeviceManagedOnly: false },
		{ value: 'network', isDeviceManagedOnly: true }
	];

	let fleetIdInput = $state('');
	let adminSession = $state<AdminSession | null>(null);
	let isDeviceReachable = $state(false);
	let activeAdminSection = $state<AdminSection>('device');

	function fleetID() {
		const explicitFleetID = fleetIdInput.trim().toLowerCase();
		if (explicitFleetID) return explicitFleetID;
		if (!browser) return '';
		return fleetIDFromHost();
	}

	function mattermostURL() {
		const providedURL = adminSession?.mattermostURL?.trim();
		if (providedURL) return providedURL;
		const currentFleetID = fleetID();
		return currentFleetID ? `https://${currentFleetID}.example.test` : '';
	}

	function adminBaseURL() {
		const currentFleetID = fleetID();
		if (fleetIDFromHost() || isMockAdminAPI) return '/admin/api';
		if (isLocalBrowserHost() && currentFleetID) return '/admin/api';
		return currentFleetID ? `https://${currentFleetID}.example.test/admin/api` : '';
	}

	const showDeviceSection = $derived(adminSession?.deviceManaged !== false);
	const currentAdminRole = $derived(adminSessionRole(adminSession));

	function adminSections(): { value: AdminSection; label: string }[] {
		return adminSectionConfigurations
			.filter((section) => showDeviceSection || !section.isDeviceManagedOnly)
			.filter((section) => canViewAdminSection(currentAdminRole, section.value))
			.map((section) => ({ value: section.value, label: text.sections[section.value] }));
	}

	$effect(() => {
		if (isVisibleAdminSection(activeAdminSection)) return;
		const visibleSection = firstVisibleAdminSection(
			currentAdminRole,
			adminSectionConfigurations.filter((section) => showDeviceSection || !section.isDeviceManagedOnly).map((section) => section.value)
		);
		if (visibleSection) activeAdminSection = visibleSection;
	});

	onMount(() => {
		const urlParams = new URLSearchParams(location.search);
		const queryFleetID = urlParams.get('fleet_id')?.trim().toLowerCase() ?? '';
		const querySection = urlParams.get('section')?.trim() ?? '';
		if (queryFleetID && !fleetIDFromHost() && !isLocalBrowserHost()) {
			location.replace(`https://${queryFleetID}.example.test/admin/`);
			return;
		}
		if (isAdminSection(querySection)) activeAdminSection = querySection;
		fleetIdInput = queryFleetID || fleetIDFromHost() || localStorage.getItem(storedFleetIdKey) || '';
		if (fleetIdInput) localStorage.setItem(storedFleetIdKey, fleetIdInput);
		loadAdminSession();
	});

	function isAdminSection(section: string): section is AdminSection {
		return adminSectionConfigurations.some((adminSection) => adminSection.value === section);
	}

	function isVisibleAdminSection(section: AdminSection) {
		return adminSections().some((adminSection) => adminSection.value === section);
	}

	function activateAdminSection(section: AdminSection) {
		activeAdminSection = section;
	}

	function fleetIDFromHost() {
		if (!browser) return '';
		const host = location.hostname;
		if (isLocalBrowserHost()) return '';
		const suffix = '.example.test';
		if (!host.endsWith(suffix)) return '';
		const currentFleetID = host.slice(0, -suffix.length);
		if (!currentFleetID || currentFleetID === 'api' || currentFleetID.includes('.')) return '';
		return currentFleetID;
	}

	function isLocalBrowserHost() {
		if (!browser) return false;
		const host = location.hostname;
		return host === 'localhost' || host === '127.0.0.1' || /^\d+\.\d+\.\d+\.\d+$/.test(host);
	}

	async function saveFleetID(savedFleetID: string) {
		if (savedFleetID) localStorage.setItem(storedFleetIdKey, savedFleetID);
		await loadAdminSession();
	}

	async function loadAdminSession() {
		if (!adminBaseURL()) return;
		try {
			adminSession = await fetchAdminSession(adminBaseURL(), '');
		} catch {
			adminSession = null;
		}
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

		{#if isVisibleAdminSection(activeAdminSection)}
			{#if activeAdminSection === 'users'}
				<Separator />
			{/if}

			<section class="grid min-w-0 gap-5 py-6">
				{#if activeAdminSection === 'device'}
					<DeviceSection
						adminBaseURL={adminBaseURL()}
						adminSession={adminSession}
						bind:fleetIdInput
						bind:isDeviceReachable
						text={text}
						onFleetIDSaved={saveFleetID}
					/>
				{:else if activeAdminSection === 'bot'}
					<BotSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
				{:else if activeAdminSection === 'credentials'}
					<CredentialsSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
				{:else if activeAdminSection === 'backup'}
					<BackupSection
						adminBaseURL={adminBaseURL()}
						fleetID={fleetID()}
						isDeviceHost={!!fleetIDFromHost()}
						isDeviceReachable={isDeviceReachable}
						text={text}
					/>
				{:else if activeAdminSection === 'users'}
					<UsersSection
						adminBaseURL={adminBaseURL()}
						adminSession={adminSession}
						fleetID={fleetID()}
						isDeviceContext={!!fleetID()}
						text={text}
						onUserChanged={loadAdminSession}
					/>
				{:else if activeAdminSection === 'settings'}
					<SettingsSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
				{:else if activeAdminSection === 'sharing'}
					<CompanyShareSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
				{:else if activeAdminSection === 'network'}
					<NetworkSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
				{/if}
			</section>
		{/if}
	</div>
</main>
