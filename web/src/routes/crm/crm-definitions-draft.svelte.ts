import { cloneCRMVocabulary, type CRMVocabulary } from './crm-definitions';

type SaveCRMVocabulary = (vocabulary: CRMVocabulary) => Promise<void> | void;

export class CRMDefinitionsDraft {
	value = $state<CRMVocabulary>({ organization_types: [], pipelines: [] });
	errorMessage = $state('');
	private isFlushing = $state(false);
	private hasPendingEdit = false;

	constructor(
		private readonly remoteVocabulary: () => CRMVocabulary,
		private readonly saveVocabulary: SaveCRMVocabulary,
		private readonly blankNameMessage: () => string
	) {}

	get isSaving(): boolean {
		return this.isFlushing;
	}

	synchronize(): void {
		const incoming = cloneCRMVocabulary(this.remoteVocabulary());
		if (this.isFlushing) return;
		this.value = incoming;
	}

	commit(next: CRMVocabulary): void {
		if (hasBlankName(next)) {
			this.errorMessage = this.blankNameMessage();
			return;
		}
		this.value = next;
		this.errorMessage = '';
		void this.flush();
	}

	private async flush(): Promise<void> {
		if (this.isFlushing) {
			this.hasPendingEdit = true;
			return;
		}
		this.isFlushing = true;
		try {
			do {
				this.hasPendingEdit = false;
				await this.saveVocabulary(cloneCRMVocabulary(this.value));
			} while (this.hasPendingEdit);
		} catch (error) {
			this.errorMessage = error instanceof Error ? error.message : String(error);
			this.value = cloneCRMVocabulary(this.remoteVocabulary());
		} finally {
			this.isFlushing = false;
		}
	}
}

function hasBlankName(vocabulary: CRMVocabulary): boolean {
	return [...vocabulary.organization_types, ...vocabulary.pipelines].some(
		(definition) => !definition.name.trim()
	);
}
