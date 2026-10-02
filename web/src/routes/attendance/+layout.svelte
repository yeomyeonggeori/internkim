<script lang="ts">
	import { page } from '$app/state';
	import { projectURL } from '$lib/supabase';
	import { companySlugOf } from '$lib/company-path';
	import AttendanceWorkspace from './attendance-workspace.svelte';

	let { children } = $props();
	const cacheScope = $derived(JSON.stringify([projectURL(), companySlugOf(page.url.pathname), page.data.session?.email ?? '']));
</script>

{#key cacheScope}
	<AttendanceWorkspace {cacheScope} {children} />
{/key}
