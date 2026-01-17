import * as React from "react";
import { Check } from "lucide-react";
import { cn } from "../../lib/utils";

export interface Step {
  number: number;
  title: string;
}

interface StepIndicatorProps {
  steps: Step[];
  currentStep: number;
  completedSteps: Set<number>;
  onStepClick?: (step: number) => void;
}

export function StepIndicator({
  steps,
  currentStep,
  completedSteps,
  onStepClick,
}: StepIndicatorProps) {
  return (
    <div className="w-full">
      <div className="flex flex-wrap items-center justify-center gap-2">
        {steps.map((step, index) => {
          const isCompleted = completedSteps.has(step.number);
          const isCurrent = currentStep === step.number;
          const isClickable = onStepClick && (isCompleted || step.number <= currentStep);

          return (
            <React.Fragment key={step.number}>
              <button
                type="button"
                onClick={() => isClickable && onStepClick(step.number)}
                disabled={!isClickable}
                className={cn(
                  "flex items-center gap-1.5 px-2 py-1 rounded-md transition-all text-xs",
                  isCurrent && "bg-primary/15 ring-1 ring-primary",
                  isCompleted && !isCurrent && "hover:bg-accent",
                  !isClickable && "cursor-not-allowed opacity-50"
                )}
              >
                <span
                  className={cn(
                    "flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-semibold transition-colors shrink-0",
                    isCurrent && "bg-primary text-primary-foreground",
                    isCompleted && !isCurrent && "bg-green-600 text-white",
                    !isCompleted && !isCurrent && "bg-secondary text-muted-foreground"
                  )}
                >
                  {isCompleted && !isCurrent ? (
                    <Check className="w-3 h-3" strokeWidth={3} />
                  ) : (
                    step.number
                  )}
                </span>
                <span
                  className={cn(
                    "font-medium whitespace-nowrap",
                    isCurrent && "text-primary",
                    isCompleted && !isCurrent && "text-green-500",
                    !isCompleted && !isCurrent && "text-muted-foreground"
                  )}
                >
                  {step.title}
                </span>
              </button>

              {index < steps.length - 1 && (
                <div
                  className={cn(
                    "w-4 h-px hidden sm:block",
                    completedSteps.has(step.number) ? "bg-green-600" : "bg-border"
                  )}
                />
              )}
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}
