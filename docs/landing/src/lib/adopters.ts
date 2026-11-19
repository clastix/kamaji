import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export interface Adopter {
  type: string;
  name: string;
  since: string;
  useCase: string;
}

// Read at build time (cwd is docs/landing) so the page never drifts from the repo's ADOPTERS.md.
export const adopters: Adopter[] = readFileSync(resolve('../../ADOPTERS.md'), 'utf8')
  .split('\n')
  .filter((l) => /^\|\s*(Vendor|End-user|R&D|Integration)\s*\|/.test(l))
  .map((l) => {
    const [, type, name, since, , useCase] = l.split('|').map((c) => c.trim());
    return { type, name, since, useCase: useCase.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1').replace(/\*\*/g, '') };
  })
  .sort((a, b) => a.name.localeCompare(b.name));
