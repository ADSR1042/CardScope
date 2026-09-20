import { fileURLToPath } from 'node:url';
import path from 'node:path';

export const root = fileURLToPath(new URL('../../../', import.meta.url));
export const runtimePath = (name) => path.join(root, '.runtime', name);
