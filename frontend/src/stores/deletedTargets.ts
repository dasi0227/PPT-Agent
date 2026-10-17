// Local deletion fences also reject responses that were already in flight.
const projects = new Set<string>();
const threads = new Set<string>();

export const isProjectDeleted = (id: string) => projects.has(id);
export const isThreadDeleted = (id: string) => threads.has(id);
export const markProjectDeleted = (id: string) => { projects.add(id); };
export const markThreadDeleted = (id: string) => { threads.add(id); };
export const restoreProject = (id: string) => { projects.delete(id); };
export const restoreThread = (id: string) => { threads.delete(id); };
