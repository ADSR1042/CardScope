import { Children, isValidElement, useState, useRef, useEffect, type ReactNode } from 'react';
import * as SelectPrimitive from '@radix-ui/react-select';
import { Check, ChevronDown, ChevronUp } from 'lucide-react';

type Props = {
  children: ReactNode;
  value?: string;
  defaultValue?: string;
  onChange?: (event: { target: { value: string } }) => void;
  'aria-label': string;
  name?: string;
  disabled?: boolean;
};

// Accept existing option markup while replacing the native OS popup everywhere.
export function Select({
  children,
  value,
  defaultValue,
  onChange,
  name,
  disabled,
  'aria-label': label,
}: Props) {
  const options = Children.toArray(children).flatMap((child) => {
    if (
      !isValidElement<{ value?: string | number; children: ReactNode; disabled?: boolean }>(child)
    )
      return [];
    return [
      {
        value: String(child.props.value ?? child.props.children),
        label: child.props.children,
        disabled: child.props.disabled,
      },
    ];
  });
  const [internal, setInternal] = useState(() => defaultValue ?? options[0]?.value ?? '');
  const inputRef = useRef<HTMLInputElement>(null);
  const resetValue = defaultValue ?? options[0]?.value ?? '';
  useEffect(() => {
    const form = inputRef.current?.form;
    if (!form || value !== undefined) return;
    const reset = () => setInternal(resetValue);
    form.addEventListener('reset', reset);
    return () => form.removeEventListener('reset', reset);
  }, [resetValue, value, name]);
  const selected = value ?? internal;
  // Radix reserves empty values for placeholders; prefix all values bijectively.
  return (
    <>
      {name && (
        <input ref={inputRef} type="hidden" name={name} value={selected} disabled={disabled} />
      )}
      <SelectPrimitive.Root
        value={`option:${selected}`}
        disabled={disabled}
        onValueChange={(encoded) => {
          const next = encoded.slice(7);
          setInternal(next);
          onChange?.({ target: { value: next } });
        }}
      >
        <SelectPrimitive.Trigger className="select-trigger" aria-label={label}>
          <SelectPrimitive.Value />
          <SelectPrimitive.Icon className="select-arrow">
            <ChevronDown size={14} />
          </SelectPrimitive.Icon>
        </SelectPrimitive.Trigger>
        <SelectPrimitive.Portal>
          <SelectPrimitive.Content
            className="select-menu"
            position="popper"
            sideOffset={6}
            collisionPadding={12}
          >
            <SelectPrimitive.ScrollUpButton className="select-scroll">
              <ChevronUp size={14} />
            </SelectPrimitive.ScrollUpButton>
            <SelectPrimitive.Viewport className="select-viewport">
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={`option:${option.value}`}
                  disabled={option.disabled}
                  className="select-option"
                >
                  <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className="select-check">
                    <Check size={15} />
                  </SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.Viewport>
            <SelectPrimitive.ScrollDownButton className="select-scroll">
              <ChevronDown size={14} />
            </SelectPrimitive.ScrollDownButton>
          </SelectPrimitive.Content>
        </SelectPrimitive.Portal>
      </SelectPrimitive.Root>
    </>
  );
}
