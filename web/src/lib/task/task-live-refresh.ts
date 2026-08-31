import type { RealtimeChannel } from '@supabase/supabase-js';
import { isSupabaseConfigured, supabase } from '$lib/supabase';

const refreshDelayMilliseconds = 400;

export function subscribeTaskWrites(onWrite: () => void): () => void {
	if (!isSupabaseConfigured()) return () => {};
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;
	let channel: RealtimeChannel | undefined;
	let isStopped = false;
	void supabase()
		.rpc('my_company_topic')
		.then(({ data }) => {
			if (isStopped || typeof data !== 'string' || !data) return;
			channel = supabase()
				.channel(data, { config: { private: true } })
				.on('broadcast', { event: 'task_written' }, () => {
					clearTimeout(refreshTimer);
					refreshTimer = setTimeout(onWrite, refreshDelayMilliseconds);
				})
				.subscribe();
		});
	return () => {
		isStopped = true;
		clearTimeout(refreshTimer);
		if (channel) void supabase().removeChannel(channel);
	};
}
