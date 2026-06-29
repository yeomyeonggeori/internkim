import {
	composeMailAddress,
	MAIL_PROVIDER_PRESETS,
	mailAddressDraftFromEmail,
	mailProviderPreset,
	type MailProviderID,
	type MailProviderPreset
} from './mail-provider-presets';
import type { MailAccountDraft } from './mail-types';

export type MailSettingsEmailState = {
	emailLocalPart: string;
	emailProviderID: MailProviderID;
	customEmailDomain: string;
	isAdvancedSettingsOpen: boolean;
};

export function mailSettingsEmailStateFromEmail(email: string): MailSettingsEmailState {
	const emailDraft = mailAddressDraftFromEmail(email);
	return {
		emailLocalPart: emailDraft.localPart,
		emailProviderID: emailDraft.providerID,
		customEmailDomain: emailDraft.customDomain,
		isAdvancedSettingsOpen: emailDraft.providerID === 'custom'
	};
}

export function selectedMailSettingsDomain(emailProviderID: MailProviderID, customEmailDomain: string): string {
	return emailProviderID === 'custom' ? customEmailDomain : mailProviderPreset(emailProviderID)?.domain || '';
}

export function mailAppPasswordInputValue(value: string, emailProviderID: MailProviderID): string {
	if (emailProviderID !== 'gmail') return value;
	return value.replace(/\s/g, '');
}

export function currentMailPresetFromDraft(accountDraft: MailAccountDraft): MailProviderPreset | undefined {
	const currentIMAPHost = accountDraft.imapHost.trim().toLowerCase();
	const currentSMTPHost = accountDraft.smtpHost.trim().toLowerCase();
	const emailPreset = mailProviderPreset(mailAddressDraftFromEmail(accountDraft.email).providerID);
	if (emailPreset?.imapHost === currentIMAPHost && emailPreset.smtpHost === currentSMTPHost) return emailPreset;
	return Object.values(MAIL_PROVIDER_PRESETS).find(
		(preset): preset is MailProviderPreset => preset !== null && preset.imapHost === currentIMAPHost && preset.smtpHost === currentSMTPHost
	);
}

export function mailProviderSettingsUpdate(accountDraft: MailAccountDraft, providerID: MailProviderID): Partial<MailAccountDraft> {
	const preset = mailProviderPreset(providerID);
	if (preset) return mailServerSettingsFromPreset(preset);
	if (!currentMailPresetFromDraft(accountDraft)) return {};
	return {
		imapHost: '',
		smtpHost: '',
		sentMailbox: 'Sent'
	};
}

export function mailAddressSettingsUpdate(
	accountDraft: MailAccountDraft,
	emailLocalPart: string,
	emailProviderID: MailProviderID,
	customEmailDomain: string,
	forceUsernameSync: boolean
): Partial<MailAccountDraft> {
	const previousEmail = accountDraft.email.trim();
	const nextEmail = composeMailAddress(emailLocalPart, selectedMailSettingsDomain(emailProviderID, customEmailDomain));
	const currentPreset = currentMailPresetFromDraft(accountDraft);
	const nextPreset = mailProviderPreset(emailProviderID);
	const previousLoginUsername = mailLoginUsernameFromEmail(previousEmail, currentPreset);
	const nextLoginUsername = mailLoginUsernameFromParts(emailLocalPart, nextEmail, nextPreset);
	const update: Partial<MailAccountDraft> = {
		email: nextEmail,
		fromAddress: nextEmail,
		...mailServerSettingsUpdateFromAddress(accountDraft, nextEmail, emailProviderID)
	};
	if (forceUsernameSync || shouldSyncMailUsername(accountDraft.imapUsername, previousLoginUsername)) {
		update.imapUsername = nextLoginUsername;
	}
	if (forceUsernameSync || shouldSyncMailUsername(accountDraft.smtpUsername, previousLoginUsername)) {
		update.smtpUsername = nextLoginUsername;
	}
	return update;
}

function mailServerSettingsUpdateFromAddress(accountDraft: MailAccountDraft, email: string, emailProviderID: MailProviderID): Partial<MailAccountDraft> {
	if (!email) return mailProviderSettingsUpdate(accountDraft, 'custom');
	const preset = mailProviderPreset(emailProviderID);
	if (!preset || !canApplyProviderPreset(accountDraft)) return {};
	return mailServerSettingsFromPreset(preset);
}

function canApplyProviderPreset(accountDraft: MailAccountDraft): boolean {
	if (currentMailPresetFromDraft(accountDraft)) return true;
	return accountDraft.imapHost.trim() === '' && accountDraft.smtpHost.trim() === '';
}

function mailServerSettingsFromPreset(preset: MailProviderPreset): Partial<MailAccountDraft> {
	return {
		imapHost: preset.imapHost,
		imapPort: preset.imapPort,
		imapSecurity: preset.imapSecurity,
		smtpHost: preset.smtpHost,
		smtpPort: preset.smtpPort,
		smtpSecurity: preset.smtpSecurity,
		sentMailbox: preset.sentMailbox
	};
}

function mailLoginUsernameFromEmail(email: string, preset: MailProviderPreset | null | undefined): string {
	const emailDraft = mailAddressDraftFromEmail(email);
	return mailLoginUsernameFromParts(emailDraft.localPart, email, preset);
}

function mailLoginUsernameFromParts(localPart: string, email: string, preset: MailProviderPreset | null | undefined): string {
	if (preset?.loginAccountMode === 'localPart') return localPart;
	return email;
}

function shouldSyncMailUsername(username: string, previousLoginUsername: string): boolean {
	const normalizedUsername = username.trim().toLowerCase();
	const normalizedPreviousLoginUsername = previousLoginUsername.trim().toLowerCase();
	return normalizedUsername === '' || (normalizedPreviousLoginUsername !== '' && normalizedUsername === normalizedPreviousLoginUsername);
}
