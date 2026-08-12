import { RefusedAccount, signIn, type MattermostSession, type MattermostSettings } from './mattermost';

export type MessengerAccount = { settings: MattermostSettings; session: MattermostSession };

export type MessengerAccountKeeper = {
	address: () => Promise<MattermostSettings>;
	admin: () => Promise<MessengerAccount>;
	forgetSession: () => void;
};

export function keepMessengerAccount(
	readConnection: () => Promise<MattermostSettings>,
	signInAs: (settings: MattermostSettings) => Promise<MattermostSession> = signIn
): MessengerAccountKeeper {
	let address: Promise<MattermostSettings> | null = null;
	let held: MessengerAccount | null = null;
	let refusedPassword: string | null = null;

	return {
		address: () => {
			if (address) return address;
			const asking = readConnection();
			address = asking;
			asking.catch(() => {
				if (address === asking) address = null;
			});
			return asking;
		},

		admin: async () => {
			const settings = await readConnection();
			if (held && held.settings.password === settings.password) return held;
			if (refusedPassword === settings.password) {
				throw new Error('the messenger refused this account; it is asked again when the password changes');
			}
			try {
				held = { settings, session: await signInAs(settings) };
				refusedPassword = null;
				return held;
			} catch (error) {
				held = null;
				if (error instanceof RefusedAccount) refusedPassword = settings.password;
				throw error;
			}
		},

		forgetSession: () => {
			held = null;
		}
	};
}
