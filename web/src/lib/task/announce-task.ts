import { supabase } from '$lib/supabase';

export async function announceTaskMoved(taskID: string): Promise<void> {
	const { data } = await supabase().auth.getSession();
	if (!data.session) return;
	await supabase()
		.functions.invoke('announce-task', { body: { taskID } })
		.catch(() => undefined);
}
