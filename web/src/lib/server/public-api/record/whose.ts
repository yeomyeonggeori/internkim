import { peopleOfHints, type RecordPerson } from './people';

export class WhoseRecordsContradicted extends Error {
	readonly errorCode = 'whose_records_contradicted';
	readonly failureStage = 'input_validation';
	readonly retryable = true;
	readonly safeRetry = true;

	constructor(readonly field: string) {
		super(
			`${field} and scope answer the same question. Name people in ${field} and leave scope out, or set scope to all and leave ${field} out.`
		);
		this.name = 'WhoseRecordsContradicted';
	}
}

// Whose records to read has three answers, and the field that carries them is
// its own tag: naming nobody is the requester, naming people is those people,
// and everyone is a constant rather than a roll call. Asking a company-wide
// read to enumerate every member would put the cost on the commonest case.
export type WhoseRecords = {
	everyone: boolean;
	personIDs: string[];
};

export function whoseRecords(
	people: RecordPerson[],
	personHints: string[] | undefined,
	scope: string | undefined,
	requesterID: string,
	field = 'personHints'
): WhoseRecords {
	const named = (personHints ?? []).filter((hint) => hint.trim() !== '');
	if (named.length > 0 && scope) throw new WhoseRecordsContradicted(field);
	if (named.length > 0) {
		return { everyone: false, personIDs: peopleOfHints(people, named).map((person) => person.personID) };
	}
	if (scope === 'all') return { everyone: true, personIDs: [] };
	return { everyone: false, personIDs: [requesterID] };
}

export function whoseRecordsHolds(whose: WhoseRecords, memberID: string | null | undefined): boolean {
	if (whose.everyone) return true;
	if (!memberID) return false;
	return whose.personIDs.includes(memberID);
}

export function whoseRecordsHoldsAny(whose: WhoseRecords, memberIDs: string[]): boolean {
	if (whose.everyone) return true;
	return memberIDs.some((memberID) => whose.personIDs.includes(memberID));
}

// The result still names one owner because that is the field its contract
// publishes; a read of several people is told apart from a company-wide one by
// scope, and by the participants the rows themselves carry.
export function ownerNamedBy(whose: WhoseRecords): string {
	return whose.personIDs.length === 1 ? whose.personIDs[0] : '';
}
