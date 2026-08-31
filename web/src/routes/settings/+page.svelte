<script lang="ts">
	import CompanyBaseCurrency from './company-base-currency.svelte';
	import CompanyProfileImage from './company-profile-image.svelte';
	import CompanyConnections from './company-connections.svelte';
	import SignInPasskeys from './sign-in-passkeys.svelte';
	import PersonalAPIKeys from './personal-access-tokens.svelte';
	import Notifications from './notifications.svelte';
	import MyMessengerAccount from './my-messenger-account.svelte';
	import OrganizationInviteDialog from '../organization/organization-invite-dialog.svelte';
	import UserPlusIcon from '@lucide/svelte/icons/user-plus';
	import { Button } from '$lib/components/ui/button';
	import AttendanceWorkSettingsSection from '../admin/attendance-work-settings-section.svelte';
	import AttendanceLeavePolicySettings from '../admin/attendance-leave-policy-settings.svelte';
	import { adminText } from '../admin/text';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';
	import { onMount } from 'svelte';

	const text = createPageText(companySettingsText);
	const attendanceSettingsText = createPageText(adminText);
	let isAdmin = $state(false);
	let isInviteOpen = $state(false);
	let isLoading = $state(isSupabaseConfigured());

	onMount(async () => {
		if (!isSupabaseConfigured()) return;
		isAdmin = (await supabaseMemberRole()) === 'admin';
		isLoading = false;
	});
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="h-full min-h-0 w-full flex-1 overflow-y-auto bg-background text-foreground">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-6 sm:px-6">
		<header class="grid gap-1">
			<h1 class="text-xl font-semibold">{text.signIn}</h1>
			<p class="text-sm text-muted-foreground">{text.signInDescription}</p>
		</header>
		<SignInPasskeys />
		<PersonalAPIKeys />
		<Notifications />
		<MyMessengerAccount />
		{#if !isLoading && isAdmin}
			<header class="grid gap-1 pt-2">
				<h2 class="text-xl font-semibold">{text.company}</h2>
				<p class="text-sm text-muted-foreground">{text.companyDescription}</p>
			</header>
			<CompanyProfileImage />
			<CompanyBaseCurrency />
			<header class="grid gap-1 pt-2">
				<h2 class="text-xl font-semibold">{text.members}</h2>
				<p class="text-sm text-muted-foreground">{text.membersDescription}</p>
			</header>
			<div>
				<Button type="button" variant="outline" onclick={() => (isInviteOpen = true)}>
					<UserPlusIcon />
					{text.inviteMember}
				</Button>
			</div>
			<OrganizationInviteDialog bind:isOpen={isInviteOpen} onInvited={() => {}} />
			<AttendanceWorkSettingsSection adminBaseURL="" text={attendanceSettingsText} />
			<AttendanceLeavePolicySettings adminBaseURL="" text={attendanceSettingsText} />
			<header class="grid gap-1 pt-2">
				<h2 class="text-xl font-semibold">{text.connections}</h2>
				<p class="text-sm text-muted-foreground">{text.connectionsDescription}</p>
			</header>
			<CompanyConnections />
		{/if}
	</div>
</main>
