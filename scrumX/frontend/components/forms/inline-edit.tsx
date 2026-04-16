'use client';

import { useEffect, useRef, useState } from 'react';
import { Input } from '@/components/ui/input';

type InlineEditProps = {
  value: string;
  onSave: (value: string) => void;
  className?: string;
  placeholder?: string;
};

export function InlineEdit({ value, onSave, className, placeholder }: InlineEditProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  useEffect(() => {
    if (!editing) return;
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [editing]);

  const save = () => {
    const normalized = draft.trim();
    if (normalized !== value.trim()) {
      onSave(normalized);
    }
    setEditing(false);
  };

  if (editing) {
    return (
      <Input
        ref={inputRef}
        className={className}
        value={draft}
        placeholder={placeholder}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={save}
        onKeyDown={(event) => {
          if (event.key === 'Enter') save();
          if (event.key === 'Escape') {
            setDraft(value);
            setEditing(false);
          }
        }}
      />
    );
  }

  return (
    <button
      type="button"
      className={`w-full rounded-md px-1 text-left hover:bg-accent ${className ?? ''}`}
      onClick={() => setEditing(true)}
    >
      {value || placeholder || 'Click to edit'}
    </button>
  );
}
