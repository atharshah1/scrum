'use client';

import { create } from 'zustand';
import { cn } from '@/lib/utils';

type ToastVariant = 'info' | 'success' | 'error';

type ToastItem = {
  id: string;
  title: string;
  description?: string;
  variant?: ToastVariant;
};

type ToastState = {
  toasts: ToastItem[];
  push: (toast: Omit<ToastItem, 'id'>) => void;
  remove: (id: string) => void;
};

const COLORS: Record<ToastVariant, string> = {
  info: 'border-blue-200 bg-blue-50 text-blue-900',
  success: 'border-emerald-200 bg-emerald-50 text-emerald-900',
  error: 'border-red-200 bg-red-50 text-red-900'
};

let toastCounter = 0;

function generateId() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID();
  }
  toastCounter += 1;
  return `${Date.now()}-${toastCounter}`;
}

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (toast) => {
    const id = generateId();
    set((state) => ({ toasts: [...state.toasts, { ...toast, id }] }));
    setTimeout(() => {
      set((state) => ({ toasts: state.toasts.filter((item) => item.id !== id) }));
    }, 4000);
  },
  remove: (id) => set((state) => ({ toasts: state.toasts.filter((item) => item.id !== id) }))
}));

export function toast(toastItem: Omit<ToastItem, 'id'>) {
  useToastStore.getState().push(toastItem);
}

export function ToastList() {
  const toasts = useToastStore((s) => s.toasts);
  const remove = useToastStore((s) => s.remove);

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[100] flex w-[360px] max-w-[calc(100vw-2rem)] flex-col gap-2">
      {toasts.map((item) => {
        const variant = item.variant ?? 'info';
        return (
          <div
            key={item.id}
            className={cn('pointer-events-auto rounded-md border p-3 shadow-md', COLORS[variant])}
            role="status"
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-medium">{item.title}</p>
                {item.description ? <p className="text-xs opacity-90">{item.description}</p> : null}
              </div>
              <button
                type="button"
                className="text-xs opacity-70 hover:opacity-100"
                onClick={() => remove(item.id)}
                aria-label="Dismiss notification"
              >
                ✕
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
