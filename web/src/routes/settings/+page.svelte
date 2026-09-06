<script lang="ts">
	import CompanyBaseCurrency from './company-base-currency.svelte';
	import CompanyProfileImage from './company-profile-image.svelte';
	import CompanyConnections from './company-connections.svelte';
	import SignInPasskeys from './sign-in-passkeys.svelte';
	import SignInPassword from './sign-in-password.svelte';
	import PersonalAPIKeys from './personal-access-tokens.svelte';
	import Notifications from './notifications.svelte';
	import MyMessengerAccount from './my-messenger-account.svelte';
	import MyAgent from './my-agent.svelte';
	import LearningSoulSection from '../admin/learning-soul-section.svelte';
	import AgentIdentitySection from '../admin/agent-identity-section.svelte';
	import CompanyMembers from './company-members.svelte';
	import AttendanceWorkSettingsSection from '../admin/attendance-work-settings-section.svelte';
	import AttendanceLeavePolicySettings from '../admin/attendance-leave-policy-settings.svelte';
	import LearnedSkillsSection from '../admin/learned-skills-section.svelte';
	import SkillsSection from '../admin/skills-section.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { adminText } from '../admin/text';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';
	import { onMount } from 'svelte';

	const text = createPageText(companySettingsText);
	const attendanceSettingsText = createPageText(adminText);
	let isAdmin = $state(false);
	let isLoading = $state(isSupabaseConfigured());
	let activeTab = $state('general');

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
	<Notifications />
	<MyMessengerAccount />
	<MyAgent />
{/snippet}

{#snippet adminSections()}
	<header class="grid gap-1">
		<h2 class="text-xl font-semibold">{text.company}</h2>
		<p class="text-sm text-muted-foreground">{text.companyDescription}</p>
	</header>
	<CompanyProfileImage />
	<CompanyBaseCurrency />
	<LearningSoulSection />
	<AgentIdentitySection />
	<header class="grid gap-1 pt-2">
		<h2 class="text-xl font-semibold">{text.members}</h2>
		<p class="text-sm text-muted-foreground">{text.membersDescription}</p>
	</header>
	<CompanyMembers />
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

<main class="h-full min-h-0 w-full flex-1 overflow-y-auto bg-background text-foreground">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-6 sm:px-6">
		{#if !isLoading && isAdmin}
			<Tabs.Root bind:value={activeTab}>
				<Tabs.List class="mb-6">
					<Tabs.Trigger value="general">{text.generalTab}</Tabs.Trigger>
					<Tabs.Trigger value="admin">{text.adminTab}</Tabs.Trigger>
				</Tabs.List>
				<Tabs.Content value="general" class="grid gap-6">
					{@render generalSections()}
				</Tabs.Content>
				<Tabs.Content value="admin" class="grid gap-6">
					{@render adminSections()}
				</Tabs.Content>
			</Tabs.Root>
		{:else}
			{@render generalSections()}
		{/if}
	</div>
</main>
