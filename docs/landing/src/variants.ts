// First option of each group is the default and the only one built for production.
export const variants = {} as const;

export type Group = keyof typeof variants;
