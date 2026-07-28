import { fetchMailMessages } from '../../routes/mail/mail-api';
import { resolveMailActorEmail } from '../../routes/mail/mail-request-actor';
import type { MailMessage } from '../../routes/mail/mail-types';

const searchResultLimit = 5;
const searchDebounceMilliseconds = 250;

class MailMessageSearch {
	results = $state<MailMessage[]>([]);
	isSearching = $state(false);
	#requestID = 0;
	#debounceTimer: ReturnType<typeof setTimeout> | undefined;

	search = (query: string) => {
		clearTimeout(this.#debounceTimer);
		const trimmedQuery = query.trim();
		if (!trimmedQuery) {
			this.#requestID += 1;
			this.results = [];
			this.isSearching = false;
			return;
		}
		this.isSearching = true;
		this.#debounceTimer = setTimeout(() => this.#fetchResults(trimmedQuery), searchDebounceMilliseconds);
	};

	reset = () => {
		clearTimeout(this.#debounceTimer);
		this.#requestID += 1;
		this.results = [];
		this.isSearching = false;
	};

	async #fetchResults(query: string) {
		this.#requestID += 1;
		const requestID = this.#requestID;
		const parameters = new URLSearchParams({ mailbox: 'INBOX', limit: String(searchResultLimit), query });
		try {
			const result = await fetchMailMessages(resolveMailActorEmail('', ''), parameters, { fallback: '', serviceUnavailable: '' });
			if (requestID !== this.#requestID) return;
			this.results = result.messages ?? [];
		} catch {
			if (requestID !== this.#requestID) return;
			this.results = [];
		} finally {
			if (requestID === this.#requestID) this.isSearching = false;
		}
	}
}

export const mailMessageSearch = new MailMessageSearch();
