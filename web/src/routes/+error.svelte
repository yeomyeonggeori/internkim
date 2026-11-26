<script lang="ts">
	import { page } from '$app/state';
	import PageFailureNotice from '$lib/components/page-failure-notice.svelte';

	const detail = $derived(`${page.status}${page.error?.message ? ` · ${page.error.message}` : ''}`);
</script>

<PageFailureNotice {detail} />
