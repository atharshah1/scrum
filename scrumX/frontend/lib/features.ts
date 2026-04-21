import type { AIIssueDraft } from '@/types';

type FeatureKey = 'AI' | 'INSIGHTS';
type FeatureFlags = Record<FeatureKey, boolean> & { AI_DEV_MOCKS: boolean };

function parseFlag(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value.toLowerCase() === 'true';
}

export const features: FeatureFlags = {
  AI: parseFlag(process.env.NEXT_PUBLIC_FEATURE_AI, false),
  INSIGHTS: parseFlag(process.env.NEXT_PUBLIC_FEATURE_INSIGHTS, true),
  AI_DEV_MOCKS: parseFlag(process.env.NEXT_PUBLIC_FEATURE_AI_DEV_MOCKS, false)
};

export const comingSoonContent: Record<FeatureKey, { title: string; description: string; hint: string }> = {
  AI: {
    title: 'Smart drafting (coming soon)',
    description: 'Draft structured issue updates from developer context while keeping your workflow deterministic and fast.',
    hint: 'Tip: scrumX already prioritizes offline safety, sync visibility, and conflict protection first.'
  },
  INSIGHTS: {
    title: 'Advanced insights (coming soon)',
    description: 'Turn raw issue history into bottleneck and delivery guidance.',
    hint: 'Tip: keep the workspace fast first; deeper reporting can layer on top later.'
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
      description: 'Rule-based preview draft generated locally while smart drafting is still gated.',
      type: 'story',
      priority: 'medium'
    },
    {
      title: 'Add acceptance criteria',
      description: 'Create concrete acceptance criteria and a validation checklist before implementation.',
      type: 'task',
      priority: 'high'
    }
  ];
}
