<script lang="ts">
	import CompanyBaseCurrency from './company-base-currency.svelte';
	import CompanyProfileImage from './company-profile-image.svelte';
	import CompanyConnections from './company-connections.svelte';
	import CompanyHost from './company-host.svelte';
	import SignInPasskeys from './sign-in-passkeys.svelte';
	import SignInPassword from './sign-in-password.svelte';
	import PersonalAPIKeys from './personal-access-tokens.svelte';
	import ConnectedApps from './connected-apps.svelte';
	import Notifications from './notifications.svelte';
	import MyMessengerAccount from './my-messenger-account.svelte';
	import MyAgent from './my-agent.svelte';
	import LearningSoulSection from '../admin/learning-soul-section.svelte';
	import AgentIdentitySection from '../admin/agent-identity-section.svelte';
	import CompanyMembers from './company-members.svelte';
	import Circles from './circles.svelte';
	import AttendanceWorkSettingsSection from '../admin/attendance-work-settings-section.svelte';
	import AttendanceLeavePolicySettings from '../admin/attendance-leave-policy-settings.svelte';
	import LearnedSkillsSection from '../admin/learned-skills-section.svelte';
	import SkillsSection from '../admin/skills-section.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { buttonVariants } from '$lib/components/ui/button';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import { adminText } from '../admin/text';
	import { companySettingsText } from './text';
	import { hostSetupText } from './setup/text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';
	import { onMount } from 'svelte';

	const text = createPageText(companySettingsText);
	const setupText = createPageText(hostSetupText);
	const attendanceSettingsText = createPageText(adminText);
	let isAdmin = $state(false);
	let isLoading = $state(isSupabaseConfigured());
	let activeTab = $state('general');
	let isMessengerAccountOpen = $state(false);

	function isAdminSession(value: unknown): boolean {
		return typeof value === 'object' && value !== null && 'isAdmin' in value && value.isAdmin === true;
	}

	onMount(async () => {
		if (!isSupabaseConfigured()) {
			const response = await fetch('/admin/api/session', { credentials: 'include' });
			isAdmin = response.ok && isAdminSession(await response.json());
			isLoading = false;
			return;
		}
		isAdmin = (await supabaseMemberRole()) === 'admin';
		isLoading = false;
	});
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

{#snippet generalSections()}
	<header class="grid gap-1">
		<h1 class="text-xl font-semibold">{text.signIn}</h1>
		<p class="text-sm text-muted-foreground">{text.signInDescription}</p>
	</header>
	<SignInPasskeys />
	{#if isSupabaseConfigured()}
		<SignInPassword />
	{/if}
	<PersonalAPIKeys />
	{#if isSupabaseConfigured()}
		<ConnectedApps />
	{/if}
	<Notifications />
	<MyAgent />
	<Collapsible.Root bind:open={isMessengerAccountOpen} class="grid gap-3">
		<Collapsible.Trigger class={buttonVariants({ variant: 'ghost', size: 'sm', class: 'w-fit' })}>
			{text.myMessengerManage}
			{#if isMessengerAccountOpen}
				<ChevronUpIcon data-icon="inline-end" />
			{:else}
				<ChevronDownIcon data-icon="inline-end" />
			{/if}
		</Collapsible.Trigger>
		{#if isMessengerAccountOpen}
			<Collapsible.Content>
				<MyMessengerAccount />
			</Collapsible.Content>
		{/if}
	</Collapsible.Root>
	{#if isSupabaseConfigured() && !isLoading && !isAdmin}
		<CompanyHost isAdmin={false} />
	{/if}
{/snippet}

{#snippet adminSections()}
	<header class="grid gap-1">
		<h2 class="text-xl font-semibold">{text.company}</h2>
		<p class="text-sm text-muted-foreground">{text.companyDescription}</p>
	</header>
	{#if isSupabaseConfigured()}
		<a href="/settings/setup" class={buttonVariants({ variant: 'outline', class: 'w-fit' })}>{setupText.title}</a>
		<CompanyHost isAdmin />
	{/if}
	<CompanyProfileImage />
	<CompanyBaseCurrency />
	<LearningSoulSection />
	<AgentIdentitySection />
	<header class="grid gap-1 pt-2">
		<h2 class="text-xl font-semibold">{text.members}</h2>
		<p class="text-sm text-muted-foreground">{text.membersDescription}</p>
	</header>
	<CompanyMembers />
	{#if isSupabaseConfigured()}<Circles />{/if}
	<AttendanceWorkSettingsSection adminBaseURL="" text={attendanceSettingsText} />
	<AttendanceLeavePolicySettings adminBaseURL="" text={attendanceSettingsText} />
	<header class="grid gap-1 pt-2">
		<h2 class="text-xl font-semibold">{text.connections}</h2>
		<p class="text-sm text-muted-foreground">{text.connectionsDescription}</p>
	</header>
	<CompanyConnections />
	<header class="grid gap-1 pt-2">
		<h2 class="text-xl font-semibold">{text.agent}</h2>
		<p class="text-sm text-muted-foreground">{text.agentDescription}</p>
	</header>
	<LearnedSkillsSection />
	<SkillsSection />
{/snippet}

<main class="h-full min-h-0 w-full flex-1 overflow-y-auto bg-background text-foreground max-sm:scroll-pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom))]">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-6 max-sm:pb-[calc(var(--app-mobile-nav-height)+var(--app-mobile-nav-bottom)+1.5rem)] sm:px-6">
		<Tabs.Root bind:value={activeTab}>
			{#if !isLoading && isAdmin}
				<Tabs.List class="mb-6">
					<Tabs.Trigger value="general">{text.generalTab}</Tabs.Trigger>
					<Tabs.Trigger value="admin">{text.adminTab}</Tabs.Trigger>
				</Tabs.List>
			{/if}
			<Tabs.Content value="general" class="grid gap-6">
				{@render generalSections()}
			</Tabs.Content>
			{#if !isLoading && isAdmin}
				<Tabs.Content value="admin" class="grid gap-6">
					{@render adminSections()}
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	</div>
</main>
