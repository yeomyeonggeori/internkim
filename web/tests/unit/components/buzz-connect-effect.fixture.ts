import { mock } from 'bun:test';
import { readFileSync } from 'node:fs';
import { compile, compileModule } from 'svelte/compiler';
import { Window } from 'happy-dom';

const browserWindow = new Window();
for (const name of Object.getOwnPropertyNames(browserWindow)) {
	if (name in globalThis) continue;
	Object.defineProperty(globalThis, name, { value: Reflect.get(browserWindow, name), configurable: true, writable: true });
}
Object.assign(globalThis, { window: browserWindow, document: browserWindow.document });
globalThis.fetch = Object.assign(async () => new Response(JSON.stringify({ relayURL: 'wss://relay.example.com' })), {
	preconnect: globalThis.fetch.preconnect
});

let ensureCalls = 0;
let stopped = 0;
let publish = () => {};
const ensure = async () => { ensureCalls += 1; await Promise.resolve(); publish(); return 'derived-key'; };
mock.module('../../../src/lib/central-buzz-identity', () => ({
	ensureCentralBuzzIdentity: ensure,
	keepCentralBuzzIdentity: () => { void ensure(); return () => { stopped += 1; }; }
}));
mock.module('../../../src/lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://central.example.com', supabase: () => ({}) }));
mock.module('../../../src/lib/buzz-relay-central-address', () => ({ centralBuzzRelayURL: async () => 'wss://relay.example.com' }));
mock.module('../../../src/lib/buzz-relay-client', () => ({ buzzPublicKeyOf: () => 'public-key' }));
mock.module('../../../src/lib/i18n/page-text.svelte', () => ({ createPageText: () => ({}) }));
mock.module('nostr-tools/nip19', () => ({ npubEncode: () => '', nsecEncode: () => '' }));

Bun.plugin({ name: 'svelte-identity-effect', setup(build) {
	build.onLoad({ filter: /buzz-connect-dialog\.svelte$/ }, ({ path }) => {
		// Mount the production script with a tiny presentation: this isolates its
		// real effect from dialog positioning and browser-only passkey controls.
		const script = (readFileSync(path, 'utf8').split('</script>')[0] + '</script>')
			.replace(/^\s*import .* from '\$lib\/components\/ui\/[^']+';/gm, '');
		return { contents: compile(script + '<p>{secretHex}</p>', { filename: path, generate: 'client' }).js.code, loader: 'js' };
	});
	build.onLoad({ filter: /\.svelte\.ts$/ }, ({ path }) => ({
		contents: compileModule(new Bun.Transpiler({ loader: 'ts' }).transformSync(readFileSync(path, 'utf8')), { filename: path, generate: 'client' }).js.code,
		loader: 'js'
	}));
	build.onLoad({ filter: /\.svelte$/ }, ({ path }) => ({
		contents: compile(readFileSync(path, 'utf8'), { filename: path, generate: 'client' }).js.code, loader: 'js'
	}));
} });

const { flushSync, mount, unmount } = await import('svelte');
const { buzzIdentity } = await import('../../../src/lib/stores/buzz-identity.svelte');
const scope = { projectURL: 'https://central.example.com', accountID: 'sample-account', companyID: 'sample-company', sessionKey: 'sample-session' };
publish = () => buzzIdentity.keepCentralSecret(scope, 'derived-key');
const { default: Dialog } = await import('../../../src/lib/components/buzz/buzz-connect-dialog.svelte');
const target = document.createElement('div');
document.body.appendChild(target);
const instance = mount(Dialog, { target, props: { open: true } });
flushSync();
await new Promise((resolve) => setTimeout(resolve, 30));
const callsAfterPublish = ensureCalls;
buzzIdentity.hideCentralSecret();
publish();
flushSync();
await new Promise((resolve) => setTimeout(resolve, 30));
const callsAfterRevalidation = ensureCalls;
await unmount(instance);
console.log(JSON.stringify({ callsAfterPublish, callsAfterRevalidation, stopped }));
