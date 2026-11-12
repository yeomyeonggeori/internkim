import type { z } from 'zod';
import type { companyEquityAddInputSchema, companyEquityDeleteInputSchema, companyEquityListInputSchema } from '../catalog/company';
import { HintRefused } from './hint-resolution';
import { refuseUnlessTheRequesterAdministers } from './people-tools';
import { fundingRecordOfHint, orNull } from './company-ledger-tools';
import type { RecordContext } from './company';
import type { CompanyEquityEntryResult, CompanyEquityListResult } from '../catalog/company';
import { InputRefused, refusalOfPostgresError } from '../refusal';

type HolderKind = CompanyEquityEntryResult['holderKind'];

export type CompanyEquityAddInput = z.infer<typeof companyEquityAddInputSchema>;

export type CompanyEquityListInput = z.infer<typeof companyEquityListInputSchema>;

export type CompanyEquityDeleteInput = z.infer<typeof companyEquityDeleteInputSchema>;

type EquityEntryRow = {
	id: string;
	effective_on: string;
	holder: string;
	holder_kind: HolderKind;
	security_class: string;
	shares: number | string;
	funding_record_id: string | null;
	note: string | null;
};

const entryColumns = 'id, effective_on, holder, holder_kind, security_class, shares, funding_record_id, note';
const onlyAnAdministratorKeepsTheLedger = 'only an administrator keeps the equity ledger';

export async function companyEquityAdd(context: RecordContext, input: CompanyEquityAddInput): Promise<CompanyEquityEntryResult> {
	if (!input.shares) throw new InputRefused('an equity line moves some shares; zero is not a line');
	const fundingRecordID = input.fundingRoundHint ? (await fundingRecordOfHint(context, input.fundingRoundHint)).id : null;
	const { data, error } = await context.caller
		.from('company_equity_entry')
		.insert({
			company_id: context.companyID,
			effective_on: input.effectiveDate,
			holder: input.holder,
			holder_kind: input.holderKind,
			security_class: orNull(input.securityClass) ?? 'common',
			shares: input.shares,
			funding_record_id: fundingRecordID,
			note: orNull(input.note)
		})
		.select(entryColumns)
		.single<EquityEntryRow>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorKeepsTheLedger);
	return answeredEntry(data);
}

export async function companyEquityList(context: RecordContext, input: CompanyEquityListInput): Promise<CompanyEquityListResult> {
	refuseUnlessTheRequesterAdministers(context, 'reads the equity ledger');
	const asOfDate = orNull(input.asOfDate) ?? context.now.toISOString().slice(0, 10);
	const entries = (await ledgerUntil(context, asOfDate)).map(answeredEntry);
	const holder = orNull(input.holder);
	return {
		asOfDate,
		...holdingsOf(entries),
		entries: holder ? entries.filter((entry) => entry.holder.includes(holder)) : entries
	};
}

export async function companyEquityDelete(context: RecordContext, input: CompanyEquityDeleteInput): Promise<CompanyEquityEntryResult> {
	const asked = input.entryHint;
	const { data, error } = await context.caller
		.from('company_equity_entry')
		.delete()
		.eq('company_id', context.companyID)
		.eq('id', asked)
		.select(entryColumns)
		.returns<EquityEntryRow[]>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorKeepsTheLedger);
	const deleted = (data ?? [])[0];
	if (!deleted) throw new HintRefused('equityEntry', asked, 'not_found', []);
	return answeredEntry(deleted);
}

export function holdingsOf(entries: CompanyEquityEntryResult[]): Pick<CompanyEquityListResult, 'totalShares' | 'holdings'> {
	const byHolding = Map.groupBy(entries, (entry) => `${entry.holder}\u0000${entry.holderKind}\u0000${entry.securityClass}`);
	const held = [...byHolding.values()]
		.map((lines) => ({
			holder: lines[0].holder,
			holderKind: lines[0].holderKind,
			securityClass: lines[0].securityClass,
			shares: lines.reduce((sum, line) => sum + line.shares, 0)
		}))
		.filter((holding) => holding.shares !== 0);
	const totalShares = held.reduce((sum, holding) => sum + holding.shares, 0);
	return {
		totalShares,
		holdings: held
			.map((holding) => ({ ...holding, percent: totalShares ? (holding.shares / totalShares) * 100 : 0 }))
			.toSorted((left, right) => right.shares - left.shares)
	};
}

async function ledgerUntil(context: RecordContext, asOfDate: string): Promise<EquityEntryRow[]> {
	const { data, error } = await context.caller
		.from('company_equity_entry')
		.select(entryColumns)
		.eq('company_id', context.companyID)
		.lte('effective_on', asOfDate)
		.order('effective_on')
		.order('created_at')
		.returns<EquityEntryRow[]>();
	if (error) throw refusalOfPostgresError(error, onlyAnAdministratorKeepsTheLedger);
	return data ?? [];
}

function answeredEntry(row: EquityEntryRow): CompanyEquityEntryResult {
	return {
		entryID: row.id,
		effectiveDate: row.effective_on,
		holder: row.holder,
		holderKind: row.holder_kind,
		securityClass: row.security_class,
		shares: Number(row.shares),
		fundingRecordID: row.funding_record_id,
		note: row.note
	};
}
