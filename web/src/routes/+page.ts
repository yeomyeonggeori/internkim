import { isSupabaseConfigured } from '$lib/supabase-session';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// The root screen manages a device. A company on the central plane has none, so
// it opens on the work the app is actually for.
export const load: PageLoad = () => {
	if (isSupabaseConfigured) redirect(307, '/flow/');
};
