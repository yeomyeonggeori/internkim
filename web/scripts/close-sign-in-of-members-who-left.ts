import { closeSignInOfMembersWhoLeft, controlPlane, type LeaverAccount } from '../src/lib/server/control-plane';

const isDryRun = process.argv.includes('--dry-run');
const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('needs SUPABASE_URL and SUPABASE_SECRET_KEY');

const settled = await closeSignInOfMembersWhoLeft(controlPlane({ projectURL, serviceRoleKey }), { isDryRun });

printAccounts(isDryRun ? 'would close' : 'closed', settled.closed);
printAccounts('already closed', settled.alreadyClosed);
console.log(`without an account: ${settled.withoutAccount.length}`);
for (const memberID of settled.withoutAccount) console.log(`  member ${memberID}`);

function printAccounts(label: string, accounts: LeaverAccount[]): void {
	console.log(`${label}: ${accounts.length}`);
	for (const account of accounts) console.log(`  member ${account.memberID} account ${account.accountID}`);
}
