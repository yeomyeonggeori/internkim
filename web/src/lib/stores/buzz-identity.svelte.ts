const STORAGE_KEY = 'internkim.buzz.secret';

function initialSecret(): string | null {
	if (typeof sessionStorage === 'undefined') return null;
	return sessionStorage.getItem(STORAGE_KEY);
}

const store = $state<{ secretHex: string | null }>({ secretHex: initialSecret() });

// The unlocked signing key is kept in sessionStorage so a page refresh in the
// same tab does not force another unlock. It clears when the tab closes.
export const buzzIdentity = {
	get secretHex(): string | null {
		return store.secretHex;
	},
	set secretHex(next: string | null) {
		store.secretHex = next;
		if (typeof sessionStorage === 'undefined') return;
		if (next) {
			sessionStorage.setItem(STORAGE_KEY, next);
		} else {
			sessionStorage.removeItem(STORAGE_KEY);
		}
	}
};
