<script lang="ts">
	import { page } from '$app/state';
	import PublicRoom from '$lib/data-room/public-room.svelte';
	import { publicDataRoomCompanyOf } from '$lib/data-room/guest';
	import { companySlugOf } from '$lib/company-path';

	function findCompany(): Promise<string> {
		return publicDataRoomCompanyOf(companySlugOf(page.url.pathname));
	}
</script>

<PublicRoom {findCompany} />
