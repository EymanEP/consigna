import { useEffect, useState, type ReactNode } from 'react';

import { Button, type ButtonVariant } from '../atoms';

interface ConfirmButtonProps {
  children: ReactNode;
  confirmLabel?: string;
  /** Accessible name, e.g. "Delete report.pdf". */
  label: string;
  onConfirm: () => void;
  variant?: ButtonVariant;
  className?: string;
}

/**
 * A two-step button for destructive actions: the first press arms it
 * ("SURE?") for a few seconds, the second one acts. Lighter than a dialog for
 * actions taken often, like deleting a file.
 */
export function ConfirmButton({
  children,
  confirmLabel = 'Sure?',
  label,
  onConfirm,
  variant = 'quiet',
  className,
}: ConfirmButtonProps) {
  const [armed, setArmed] = useState(false);

  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 3000);
    return () => clearTimeout(t);
  }, [armed]);

  return (
    <Button
      variant={armed ? 'danger' : variant}
      className={className}
      aria-label={armed ? `Confirm: ${label}` : label}
      onClick={() => {
        if (armed) {
          setArmed(false);
          onConfirm();
        } else {
          setArmed(true);
        }
      }}
    >
      {armed ? confirmLabel : children}
    </Button>
  );
}
