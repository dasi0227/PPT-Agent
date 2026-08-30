import { fetchClient } from './client';
import type { SkillsResponse } from './types';

export const skillsApi = {
  list: () => fetchClient<SkillsResponse>('/skills', { reportError: false }),
};
