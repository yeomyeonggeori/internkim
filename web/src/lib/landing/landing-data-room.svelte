<script lang="ts">
	import CompanyOverview from '$lib/data-room/company-overview.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { sampleCompany } from './landing-samples';

	const company = $derived(sampleCompany(currentLocale.value, new Date()));
</script>

<div class="max-h-[50rem] overflow-hidden p-4 [mask-image:linear-gradient(to_bottom,black_88%,transparent)] sm:p-6">
	<CompanyOverview {company} />
</div>
