import type { MailProviderID } from './mail-provider-presets';
import type { mailText } from './text';

type MailText = (typeof mailText)[keyof typeof mailText];

export type MailProviderSetupGuideImage = {
	src: string;
	alt: string;
};

export type MailProviderSetupGuideItem = {
	href: string;
	title: string;
	notes: string[];
	image: MailProviderSetupGuideImage;
};

export type MailProviderSetupGuide = {
	panelID: string;
	triggerLabel: string;
	isNumbered: boolean;
	items: MailProviderSetupGuideItem[];
};

export function mailProviderSetupGuide(providerID: MailProviderID, text: MailText): MailProviderSetupGuide | null {
	if (providerID === 'gmail') return googleSetupGuide(text);
	if (providerID === 'naver') return naverSetupGuide(text);
	if (providerID === 'daum' || providerID === 'hanmail') return daumSetupGuide(text);
	return null;
}

function googleSetupGuide(text: MailText): MailProviderSetupGuide {
	return {
		panelID: 'mail-google-setup',
		triggerLabel: text.settingsSheet.googleAppPasswordLink,
		isNumbered: false,
		items: [
			{
				href: 'https://myaccount.google.com/apppasswords',
				title: text.settingsSheet.googleAppPasswordLink,
				notes: [text.settingsSheet.googleAccountNote, text.settingsSheet.googleSetupNote],
				image: {
					src: '/mail/providers/google/app-password-created-example.png',
					alt: text.settingsSheet.setupImageAlts.googleAppPasswordCreated
				}
			}
		]
	};
}

function naverSetupGuide(text: MailText): MailProviderSetupGuide {
	return {
		panelID: 'mail-naver-setup-links',
		triggerLabel: text.settingsSheet.naverSetupLink,
		isNumbered: true,
		items: [
			{
				href: 'https://mail.naver.com/v2/settings/smtp/imap',
				title: text.settingsSheet.naverSetupSteps[0],
				notes: [text.settingsSheet.naverAccountNote, text.settingsSheet.naverSetupNotes[0]],
				image: {
					src: '/mail/providers/naver/imap-smtp-example.png',
					alt: text.settingsSheet.setupImageAlts.naverIMAPSMTP
				}
			},
			{
				href: 'https://nid.naver.com/user2/help/2StepVerif',
				title: text.settingsSheet.naverSetupSteps[1],
				notes: [text.settingsSheet.naverSetupNotes[1]],
				image: {
					src: '/mail/providers/naver/app-password-example.png',
					alt: text.settingsSheet.setupImageAlts.naverAppPassword
				}
			}
		]
	};
}

function daumSetupGuide(text: MailText): MailProviderSetupGuide {
	return {
		panelID: 'mail-daum-hanmail-setup-links',
		triggerLabel: text.settingsSheet.daumSetupLink,
		isNumbered: true,
		items: [
			{
				href: 'https://mail.daum.net/setting/POP3IMAP',
				title: text.settingsSheet.daumSetupSteps[0],
				notes: [text.settingsSheet.daumSetupNotes[0]],
				image: {
					src: '/mail/providers/daum/imap-smtp-example.png',
					alt: text.settingsSheet.setupImageAlts.daumIMAPSMTP
				}
			},
			{
				href: 'https://member.daum.net/my/security',
				title: text.settingsSheet.daumSetupSteps[1],
				notes: [text.settingsSheet.daumSetupNotes[1]],
				image: {
					src: '/mail/providers/daum/app-password-example.png',
					alt: text.settingsSheet.setupImageAlts.daumAppPassword
				}
			}
		]
	};
}
