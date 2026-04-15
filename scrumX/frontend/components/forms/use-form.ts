'use client';

import { useMemo, useState } from 'react';

export type ValidatorFn = (value: string) => string | null;
export type ValidationSchema<T extends string> = Partial<Record<T, ValidatorFn[]>>;

export function required(label: string): ValidatorFn {
  return (value) => (value.trim() ? null : `${label} is required`);
}

export function minLength(label: string, length: number): ValidatorFn {
  return (value) => (value.trim().length >= length ? null : `${label} must be at least ${length} characters`);
}

export function email(label: string): ValidatorFn {
  const re = /^\S+@\S+\.\S+$/;
  return (value) => (re.test(value.trim()) ? null : `${label} is invalid`);
}

export function useFormFields<T extends string>(
  initial: Record<T, string>,
  schema: ValidationSchema<T> = {}
) {
  const [values, setValues] = useState<Record<T, string>>(initial);
  const [errors, setErrors] = useState<Partial<Record<T, string>>>({});

  const setField = (field: T, value: string) => {
    setValues((prev) => ({ ...prev, [field]: value }));
    setErrors((prev) => ({ ...prev, [field]: undefined }));
  };

  const validate = () => {
    const nextErrors: Partial<Record<T, string>> = {};
    (Object.keys(schema) as T[]).forEach((field) => {
      const validators = schema[field] ?? [];
      for (const validator of validators) {
        const result = validator(values[field] ?? '');
        if (result) {
          nextErrors[field] = result;
          break;
        }
      }
    });
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  };

  const isValid = useMemo(() => {
    const keys = Object.keys(schema) as T[];
    if (!keys.length) return true;
    return keys.every((field) => {
      const validators = schema[field] ?? [];
      return validators.every((validator) => !validator(values[field] ?? ''));
    });
  }, [schema, values]);

  const reset = (next?: Partial<Record<T, string>>) => {
    setValues({ ...initial, ...next } as Record<T, string>);
    setErrors({});
  };

  return { values, errors, setField, validate, isValid, reset };
}
