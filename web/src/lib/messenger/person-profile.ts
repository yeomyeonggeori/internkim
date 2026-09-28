import type { MentionPerson } from './mention-candidates';

export const personProfileContext = Symbol('person-profile');

export type PersonProfileActions = {
	show: (person: MentionPerson) => void;
	message: (person: MentionPerson) => void;
};
