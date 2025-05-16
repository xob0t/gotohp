// Index.ts file for Switch component variants
import { cva, type VariantProps } from "class-variance-authority";

export { default as Switch } from "./Switch.vue";

export const switchVariants = cva(
  "peer inline-flex shrink-0 items-center rounded-full border border-transparent shadow-xs transition-all outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "data-[state=checked]:bg-primary data-[state=unchecked]:bg-input dark:data-[state=unchecked]:bg-input/80",
        success: "data-[state=checked]:bg-success data-[state=unchecked]:bg-input dark:data-[state=unchecked]:bg-input/80",
        destructive: "data-[state=checked]:bg-destructive data-[state=unchecked]:bg-input dark:data-[state=unchecked]:bg-input/80",
        outline: "border-border data-[state=checked]:bg-primary data-[state=unchecked]:bg-background",
      },
      size: {
        default: "h-[1.15rem] w-8",
        sm: "h-4 w-7",
        lg: "h-6 w-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export const switchThumbVariants = cva(
  "pointer-events-none block rounded-full ring-0 transition-transform data-[state=checked]:translate-x-[calc(100%-2px)] data-[state=unchecked]:translate-x-0",
  {
    variants: {
      variant: {
        default: "bg-background dark:data-[state=unchecked]:bg-foreground dark:data-[state=checked]:bg-primary-foreground",
        success: "bg-background dark:data-[state=unchecked]:bg-foreground dark:data-[state=checked]:bg-success-foreground",
        destructive: "bg-destructive-foreground dark:data-[state=unchecked]:bg-foreground dark:data-[state=checked]:bg-destructive-foreground",
        outline: "bg-foreground dark:data-[state=checked]:bg-primary-foreground",
      },
      size: {
        default: "size-4",
        sm: "size-3",
        lg: "size-5",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export type SwitchVariants = VariantProps<typeof switchVariants>;
export type SwitchThumbVariants = VariantProps<typeof switchThumbVariants>;
