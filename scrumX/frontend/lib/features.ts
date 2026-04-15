import type { AIIssueDraft } from '@/types';

type FeatureKey = 'AI' | 'INSIGHTS';

function parseFlag(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value.toLowerCase() === 'true';
}

export const features: Record<FeatureKey, boolean> & { AI_DEV_MOCKS: boolean } = {
  AI: parseFlag(process.env.NEXT_PUBLIC_FEATURE_AI, false),
  INSIGHTS: parseFlag(process.env.NEXT_PUBLIC_FEATURE_INSIGHTS, true),
  AI_DEV_MOCKS: parseFlag(process.env.NEXT_PUBLIC_FEATURE_AI_DEV_MOCKS, false)
};

export const comingSoonContent: Record<FeatureKey, { title: string; description: string; hint: string }> = {
  AI: {
    title: 'AI Assistant (Coming soon)',
    description: 'Auto-summarize work and suggest structured issue updates in one click.',
    hint: 'Tip: we will keep your existing board/issue flows fast and fully stable while this rolls out.'
  },
  INSIGHTS: {
    title: 'Advanced Insights (Coming soon)',
    description: 'Turn raw issue history into bottleneck and delivery guidance.',
    hint: 'Tip: initial velocity, bottleneck, and stuck-task cards are available today.'
  }
};

export function getAIIssueDraftMocks(sourceText: string): AIIssueDraft[] {
  if (features.AI || !features.AI_DEV_MOCKS) {
    return [];
  }
  const trimmed = sourceText.trim();
  if (!trimmed) return [];
  return [
    {
      title: `Refine: ${trimmed.slice(0, 40)}`,
      description: 'Mock draft generated in dev mode while AI feature flag is disabled.',
      type: 'story',
      priority: 'medium'
    },
    {
      title: 'Add acceptance criteria',
      description: 'Create concrete acceptance criteria and validation checklist.',
      type: 'task',
      priority: 'high'
    }
  ];
}
