import { forwardRef } from 'react';
import { Loader2 } from 'lucide-react';
import { Button as ShadcnButton, ButtonProps as ShadcnButtonProps } from '../ui/button';
import { cn } from '../../lib/utils';

interface ButtonProps extends Omit<ShadcnButtonProps, 'variant'> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'outline' | 'link';
  loading?: boolean;
}

const variantMap = {
  primary: 'default',
  secondary: 'secondary',
  danger: 'destructive',
  ghost: 'ghost',
  outline: 'outline',
  link: 'link',
} as const;

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', loading, children, disabled, className, type = 'button', ...props }, ref) => {
    const mappedVariant = variantMap[variant] || 'default';

    return (
      <ShadcnButton
        ref={ref}
        type={type}
        variant={mappedVariant as any}
        disabled={disabled || loading}
        className={cn(className)}
        {...props}
      >
        {loading && <Loader2 className="h-4 w-4 animate-spin" />}
        {children}
      </ShadcnButton>
    );
  }
);

Button.displayName = 'Button';
export default Button;
