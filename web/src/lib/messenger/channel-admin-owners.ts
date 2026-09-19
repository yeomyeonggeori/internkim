import { addChannelOwner } from './messenger-api';
import { externalIDsOfMember, fetchMessengerDirectory, type MessengerDirectory } from './messenger-directory';

export async function seatCompanyAdminsAsOwners(channelID: string): Promise<string[]> {
	const directory = await fetchMessengerDirectory();
	return seatAdminsFromDirectory(directory, (externalID) => addChannelOwner(channelID, externalID));
}

export async function seatAdminsFromDirectory(
	directory: MessengerDirectory,
	seatOwner: (externalID: string) => Promise<void>
): Promise<string[]> {
	const unseatedAdminNames: string[] = [];
	for (const memberID of directory.adminMemberIDs) {
		const displayName = directory.nameOfMember.get(memberID) ?? '';
		const externalID = externalIDsOfMember(directory, memberID)[0];
		if (!externalID) {
			unseatedAdminNames.push(displayName);
			continue;
		}
		try {
			await seatOwner(externalID);
		} catch {
			unseatedAdminNames.push(displayName);
		}
	}
	return unseatedAdminNames;
}
