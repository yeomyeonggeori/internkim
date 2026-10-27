import { toast } from 'svelte-sonner';

export function noticeWhenChannelOrderFails(change: Promise<void>, notice: string): void {
	change.catch((failure: unknown) => {
		console.warn('the channel order did not change', failure);
		toast.error(notice);
	});
}
