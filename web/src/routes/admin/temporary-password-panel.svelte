<script lang="ts">
	import { CopyButton } from '$lib/components/ui/copy-button';
	import type { AdminPageText } from './admin-types';

	type TemporaryPasswordPanelProps = {
		text: AdminPageText['users'];
		email: string;
		password: string;
	};

	let { text, email, password }: TemporaryPasswordPanelProps = $props();
</script>

<div class="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<p class="font-semibold">{text.temporaryPasswordTitle}: {email} / {password}</p>
			<p class="mt-1">{text.temporaryPasswordNotice}</p>
			<code class="mt-3 block rounded-md bg-white px-3 py-2 font-mono text-base">{password}</code>
		</div>
		<CopyButton text={password} />
	</div>
</div>
