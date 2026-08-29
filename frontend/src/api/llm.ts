import { fetchClient } from './client';
import type { LLMProfilesResponse } from './types';

export const llmApi = {
  profiles: () => fetchClient<LLMProfilesResponse>('/llm/profiles', { reportError: false }),
};
