import { expect, test } from 'bun:test';

import commandSet from '../../../../internal/desktop/commands.json';
import { desktopPrograms } from '../../../src/lib/server/public-api/catalog/desktop';

test('the desktop tools offer exactly the programs the command set describes', () => {
	const offered: string[] = [...desktopPrograms];
	expect(offered.sort()).toEqual(Object.keys(commandSet.programs).sort());
});
