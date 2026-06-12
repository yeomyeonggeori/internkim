<script lang="ts">
	import { browser } from '$app/environment';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import {
		createCompanionPairingCode,
		fetchCompanionReleases,
		fetchCompanions,
		revokeCompanion
	} from './admin-api';
	import type { AdminPageText, CompanionPairingCodeResponse, CompanionRelease, CompanionStatus } from './admin-types';

	type CompanionSectionProps = {
		adminBaseURL: string;
		companionReleaseURL: string;
		isDeviceReachable: boolean;
		mattermostURL: string;
		text: AdminPageText;
	};

	let { adminBaseURL, companionReleaseURL, isDeviceReachable, mattermostURL, text }: CompanionSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let loadedCompanionReleaseURL = $state('');
	let companionStatuses = $state<CompanionStatus[]>([]);
	let companionReleases = $state<CompanionRelease[]>([]);
	let companionPairingCode = $state<CompanionPairingCodeResponse | null>(null);
	let companionErrorMessage = $state('');
	let isLoadingCompanions = $state(false);
	let isCreatingPairingCode = $state(false);

	$effect(() => {
		if (!companionReleaseURL || loadedCompanionReleaseURL === companionReleaseURL) return;
		loadedCompanionReleaseURL = companionReleaseURL;
		loadCompanionReleases();
	});

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadCompanions();
	});

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

	async function loadCompanionReleases() {
		try {
			const response = await fetchCompanionReleases(companionReleaseURL, text.companion.loadingDownloads);
			companionReleases = response.platforms ?? [];
		} catch {
			companionReleases = [];
		}
	}

	async function loadCompanions() {
		if (!adminBaseURL) return;

		isLoadingCompanions = true;
		companionErrorMessage = '';
		try {
			const response = await fetchCompanions(adminBaseURL, text.messages.companionStatusError);
			companionStatuses = response.companions ?? [];
		} catch {
			companionErrorMessage = text.messages.companionStatusError;
		} finally {
			isLoadingCompanions = false;
		}
	}

	async function createPairingCode() {
		if (!adminBaseURL) return;

		isCreatingPairingCode = true;
		companionErrorMessage = '';
		try {
			companionPairingCode = await createCompanionPairingCode(adminBaseURL, text.messages.companionPairingError);
			if (companionPairingCode.deepLink && browser) location.href = companionPairingCode.deepLink;
		} catch {
			companionErrorMessage = text.messages.companionPairingError;
		} finally {
			isCreatingPairingCode = false;
		}
	}

	async function revokeCompanionConnection(companionID: string) {
		if (!adminBaseURL) return;

		companionErrorMessage = '';
		try {
			await revokeCompanion(adminBaseURL, companionID, text.messages.companionRevokeError);
			await loadCompanions();
		} catch {
			companionErrorMessage = text.messages.companionRevokeError;
		}
	}
</script>

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
			<Button disabled={!isDeviceReachable || isCreatingPairingCode} onclick={createPairingCode} class="gap-2">
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
							text={`internkim-companion pair --device-url ${mattermostURL} --code ${companionPairingCode.code}`}
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
					<Button variant="ghost" size="sm" onclick={() => revokeCompanionConnection(companion.companionID)}>{text.companion.revoke}</Button>
				</div>
			{/each}
		{/if}
	</div>
</div>
