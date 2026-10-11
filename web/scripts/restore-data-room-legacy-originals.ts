import { controlPlane } from '../src/lib/server/control-plane';
import { restoreLegacyOriginals } from '../src/lib/server/data-room-legacy-originals';

const isDryRun = process.argv.includes('--dry-run');
const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('needs SUPABASE_URL and SUPABASE_SECRET_KEY');

const restoration = await restoreLegacyOriginals(controlPlane({ projectURL, serviceRoleKey }), { isDryRun });

console.log(`${isDryRun ? 'would refile' : 'refiled'}: ${restoration.restored.length}`);
for (const original of restoration.restored) {
	console.log(`  ${original.title}\n    ${original.from}\n    -> ${original.to}`);
	for (const derived of original.derived) console.log(`    -> ${derived.to}`);
}
console.log(`no legacy original with the document's hash: ${restoration.unmatched.length}`);
for (const document of restoration.unmatched) console.log(`  ${document.documentID} ${document.title}`);
