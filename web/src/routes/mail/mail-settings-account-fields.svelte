<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		DEFAULT_MAIL_PROVIDER_ID,
		type MailProviderID
	} from './mail-provider-presets';
	import MailProviderSetupGuide from './mail-provider-setup-guide.svelte';
	import { mailProviderSetupGuide } from './mail-provider-setup-guides';
	import type { MailAccount, MailAccountDraft } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		accountDraft: MailAccountDraft;
		emailLocalPart: string;
		emailProviderID: MailProviderID;
		customEmailDomain: string;
		text: (typeof mailText)['ko'];
		handleEmailProviderChange: (event: Event) => void;
		syncAccountEmailFromParts: (forceUsernameSync: boolean) => void;
		syncCommonAppPassword: (event: Event) => void;
	};

	let {
		account,
		accountDraft = $bindable(),
		emailLocalPart = $bindable(''),
		emailProviderID = $bindable(DEFAULT_MAIL_PROVIDER_ID),
		customEmailDomain = $bindable(''),
		text,
		handleEmailProviderChange,
		syncAccountEmailFromParts,
		syncCommonAppPassword
	}: Props = $props();
	let openSetupProviderID = $state<MailProviderID | null>(null);
	let setupGuide = $derived(mailProviderSetupGuide(emailProviderID, text));
	let isSetupGuideOpen = $derived(setupGuide !== null && openSetupProviderID === emailProviderID);

	function toggleSetupGuide() {
		openSetupProviderID = isSetupGuideOpen ? null : emailProviderID;
	}
</script>

<div class="grid gap-4">
	<div class="space-y-2 sm:col-span-2">
		<Label for="mail-email">{text.fields.emailAddress}</Label>
		<div class="grid grid-cols-[minmax(0,1fr)_auto_minmax(120px,0.8fr)] items-center gap-2">
			<Input
				id="mail-email"
				bind:value={emailLocalPart}
				placeholder="example"
				oninput={() => syncAccountEmailFromParts(false)}
			/>
			<span class="text-sm font-medium text-muted-foreground">@</span>
			<select
				id="mail-email-provider"
				aria-label={text.fields.emailDomain}
				class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm"
				bind:value={emailProviderID}
				onchange={handleEmailProviderChange}
			>
				<option value="gmail">{text.providers.gmail}</option>
				<option value="naver">{text.providers.naver}</option>
				<option value="daum">{text.providers.daum}</option>
				<option value="hanmail">{text.providers.hanmail}</option>
				<option value="custom">{text.providers.custom}</option>
			</select>
		</div>
		{#if emailProviderID === 'custom'}
			<div class="space-y-2">
				<Label for="mail-custom-domain">{text.fields.customDomain}</Label>
				<Input
					id="mail-custom-domain"
					bind:value={customEmailDomain}
					placeholder="example.com"
					oninput={() => syncAccountEmailFromParts(false)}
				/>
			</div>
		{/if}
		<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.emailAddress}</p>
	</div>
	<div class="grid w-full gap-3 sm:col-span-2">
		<div class="w-full space-y-2">
			<div class="flex items-center justify-between gap-3">
				<Label for="mail-app-password">{text.fields.appPassword}</Label>
				{#if setupGuide}
					<button
						type="button"
						class="text-xs font-medium text-primary underline-offset-4 hover:underline"
						aria-expanded={isSetupGuideOpen}
						aria-controls={setupGuide.panelID}
						onclick={toggleSetupGuide}
					>
						{setupGuide.triggerLabel}
					</button>
				{/if}
			</div>
				<Input
					id="mail-app-password"
					type="password"
					autocomplete="current-password"
					value={accountDraft.imapPassword}
					placeholder={account.hasIMAPPassword || account.hasSMTPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword}
					oninput={syncCommonAppPassword}
				/>
			{#if setupGuide && isSetupGuideOpen}
				<MailProviderSetupGuide guide={setupGuide} />
			{/if}
			<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.appPassword}</p>
		</div>
		<div class="w-full space-y-2">
			<Label for="mail-display-name">{text.fields.displayName}</Label>
			<Input id="mail-display-name" bind:value={accountDraft.displayName} placeholder={text.settingsSheet.displayNamePlaceholder} />
			<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.displayName}</p>
		</div>
	</div>
</div>
