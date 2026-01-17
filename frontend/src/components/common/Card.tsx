import { ReactNode } from 'react';
import {
  Card as ShadcnCard,
  CardHeader,
  CardTitle,
  CardContent,
} from '../ui/card';
import { cn } from '../../lib/utils';

interface CardProps {
  children: ReactNode;
  className?: string;
  title?: string;
  actions?: ReactNode;
}

export default function Card({ children, className = '', title, actions }: CardProps) {
  if (title || actions) {
    return (
      <ShadcnCard className={cn("bg-card border-border", className)}>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 px-4 py-3">
          {title && <CardTitle className="text-base font-semibold">{title}</CardTitle>}
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </CardHeader>
        <CardContent className="px-4 pb-4 pt-0">{children}</CardContent>
      </ShadcnCard>
    );
  }

  return (
    <ShadcnCard className={cn("bg-card border-border", className)}>
      <CardContent className="p-4">{children}</CardContent>
    </ShadcnCard>
  );
}
