<script lang="ts">
	import { browser } from '$app/environment';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { fetchAdminSession } from './admin/admin-api';
	import { adminSessionRole, canViewAdminSection, firstVisibleAdminSection } from './admin/admin-role-policy';
	import APITokenSection from './admin/api-token-section.svelte';
	import BackupSection from './admin/backup-section.svelte';
	import BotSection from './admin/bot-section.svelte';
	import BuzzSection from './admin/buzz-section.svelte';
	import CredentialsSection from './admin/credentials-section.svelte';
	import CompanyShareSection from './admin/company-share-section.svelte';
	import DeviceSection from './admin/device-section.svelte';
	import NetworkSection from './admin/network-section.svelte';
	import SettingsSection from './admin/settings-section.svelte';
	import type { AdminSection, AdminSession } from './admin/admin-types';
	import { adminText } from './admin/text';
	import UsersSection from './admin/users-section.svelte';

	const storedFleetIdKey = 'internkim_fleet_id';
	const isMockAdminAPI = import.meta.env.VITE_MOCK_ADMIN === '1';
	const mockAdminEmail = import.meta.env.VITE_DEV_USER_EMAIL?.trim() || 'kim@example.com';
	const text = createPageText(adminText);
	const adminSectionConfigurations: { value: AdminSection; isDeviceManagedOnly: boolean }[] = [
		{ value: 'device', isDeviceManagedOnly: true },
		{ value: 'users', isDeviceManagedOnly: false },
		{ value: 'credentials', isDeviceManagedOnly: false },
		{ value: 'backup', isDeviceManagedOnly: false },
		{ value: 'bot', isDeviceManagedOnly: false },
		{ value: 'settings', isDeviceManagedOnly: false },
		{ value: 'sharing', isDeviceManagedOnly: false },
		{ value: 'network', isDeviceManagedOnly: true },
		{ value: 'buzz', isDeviceManagedOnly: false },
		{ value: 'apiTokens', isDeviceManagedOnly: false }
	];

	let fleetIdInput = $state('');
	let adminSession = $state<AdminSession | null>(null);
	let isAdminSessionLoaded = $state(false);
	let isDeviceReachable = $state(isMockAdminAPI);
	let activeAdminSection = $state<AdminSection>('device');

	function fleetID() {
		const explicitFleetID = fleetIdInput.trim().toLowerCase();
		if (explicitFleetID) return explicitFleetID;
		if (!browser) return '';
		return fleetIDFromHost();
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
		if (!isAdminSessionLoaded) return;
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
			const loadedSession = await fetchAdminSession(adminBaseURL(), '');
			adminSession = isMockAdminAPI ? { ...loadedSession, email: mockAdminEmail, isAdmin: true, role: 'admin' } : loadedSession;
		} catch {
			adminSession = null;
		} finally {
			isAdminSessionLoaded = true;
		}
	}
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="bg-background text-foreground min-h-svh w-full min-w-0 flex-1">
	<div class="mx-auto flex min-h-svh w-full max-w-6xl min-w-0 flex-col px-4 py-5 sm:px-5 sm:py-6">
		<header class="flex flex-wrap items-center justify-between gap-4">
			<h1 class="text-xl font-semibold">{text.pageTitle}</h1>
		</header>

		{#if isAdminSessionLoaded}
			<UnderlineTabs.Root class="pt-4" value={activeAdminSection} onValueChange={(section) => activateAdminSection(section as AdminSection)}>
				<UnderlineTabs.List>
					{#each adminSections() as section (section.value)}
						<UnderlineTabs.Trigger value={section.value}>{section.label}</UnderlineTabs.Trigger>
					{/each}
				</UnderlineTabs.List>
			</UnderlineTabs.Root>

			{#if isVisibleAdminSection(activeAdminSection)}
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
					{:else if activeAdminSection === 'buzz'}
						<BuzzSection adminBaseURL={adminBaseURL()} isDeviceReachable={isDeviceReachable} text={text} />
					{:else if activeAdminSection === 'apiTokens'}
						<APITokenSection />
					{/if}
				</section>
			{/if}
		{/if}
	</div>
</main>
