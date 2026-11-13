<script lang="ts">
	import { page } from '$app/state';
	import { z } from 'zod';
	import PublicRoom from '$lib/data-room/public-room.svelte';

	async function findCompany(): Promise<string> {
		return z.guid().parse(page.params.companyID);
	}
</script>

<PublicRoom {findCompany} />
