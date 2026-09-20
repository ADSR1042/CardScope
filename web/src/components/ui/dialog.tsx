import * as React from 'react';
import * as Primitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
export const Dialog = Primitive.Root;
export const DialogTitle = Primitive.Title;
export const DialogDescription = Primitive.Description;
export function DialogContent({
  children,
  wide = false,
  ...props
}: React.ComponentPropsWithoutRef<typeof Primitive.Content> & { wide?: boolean }) {
  return (
    <Primitive.Portal>
      <Primitive.Overlay className="dialog-overlay" />
      <Primitive.Content className={wide ? 'dialog-content drawer' : 'dialog-content'} {...props}>
        {children}
        <Primitive.Close className="close-dialog" aria-label="关闭">
          <X size={18} />
        </Primitive.Close>
      </Primitive.Content>
    </Primitive.Portal>
  );
}
