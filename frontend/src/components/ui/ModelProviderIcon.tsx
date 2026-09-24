import { Cpu } from 'lucide-react';

const logos: Record<string, string> = {
  openai: 'openai', anthropic: 'claude', deepseek: 'deepseek', kimi: 'kimi',
  mimo: 'mimo', gemini: 'gemini', qwen: 'qwen', minimax: 'minimax', zai: 'zai',
};

/** Branding follows provider, regardless of the wire protocol or gateway URL. */
export function ModelProviderIcon({ provider, className, size = 16 }: { provider?: string; className?: string; size?: number }) {
  const logo = provider ? logos[provider] : undefined;
  return logo
    ? <img src={`/model-logos/${logo}.svg`} alt="" aria-hidden="true" width={size} height={size} className={className} />
    : <Cpu aria-hidden="true" size={size} className={className} />;
}
