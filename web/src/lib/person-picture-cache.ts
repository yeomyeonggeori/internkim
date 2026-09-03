export type CachedPicture = { externalID: string; avatarURL: string; dataURL: string };

const databaseName = 'internkim-person-pictures';
const storeName = 'pictures';

let database: Promise<IDBDatabase | null> | null = null;

function openDatabase(): Promise<IDBDatabase | null> {
	if (typeof indexedDB === 'undefined') return Promise.resolve(null);
	database ??= new Promise<IDBDatabase | null>((resolve) => {
		try {
			const request = indexedDB.open(databaseName, 1);
			request.onupgradeneeded = () => {
				if (request.result.objectStoreNames.contains(storeName)) return;
				request.result.createObjectStore(storeName, { keyPath: 'externalID' });
			};
			request.onsuccess = () => resolve(request.result);
			request.onerror = () => resolve(null);
			request.onblocked = () => resolve(null);
		} catch {
			resolve(null);
		}
	});
	return database;
}

export async function readCachedPictures(): Promise<CachedPicture[]> {
	const opened = await openDatabase();
	if (!opened) return [];
	return new Promise((resolve) => {
		try {
			const request = opened.transaction(storeName, 'readonly').objectStore(storeName).getAll();
			request.onsuccess = () => resolve(request.result as CachedPicture[]);
			request.onerror = () => resolve([]);
		} catch {
			resolve([]);
		}
	});
}

export async function writeCachedPicture(picture: CachedPicture): Promise<void> {
	return writeToStore((store) => store.put(picture));
}

export async function forgetCachedPicture(externalID: string): Promise<void> {
	return writeToStore((store) => store.delete(externalID));
}

async function writeToStore(change: (store: IDBObjectStore) => void): Promise<void> {
	const opened = await openDatabase();
	if (!opened) return;
	return new Promise((resolve) => {
		try {
			const transaction = opened.transaction(storeName, 'readwrite');
			change(transaction.objectStore(storeName));
			transaction.oncomplete = () => resolve();
			transaction.onerror = () => resolve();
			transaction.onabort = () => resolve();
		} catch {
			resolve();
		}
	});
}
