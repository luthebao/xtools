import { Toaster as Sonner } from "sonner"

type ToasterProps = React.ComponentProps<typeof Sonner>

const Toaster = ({ ...props }: ToasterProps) => {
  return (
    <Sonner
      theme="dark"
      className="toaster group"
      position="bottom-right"
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-card group-[.toaster]:text-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg",
          description: "group-[.toast]:text-muted-foreground",
          actionButton:
            "group-[.toast]:bg-primary group-[.toast]:text-primary-foreground",
          cancelButton:
            "group-[.toast]:bg-muted group-[.toast]:text-muted-foreground",
          success: "group-[.toaster]:bg-green-950 group-[.toaster]:border-green-900",
          error: "group-[.toaster]:bg-red-950 group-[.toaster]:border-red-900",
          warning: "group-[.toaster]:bg-yellow-950 group-[.toaster]:border-yellow-900",
          info: "group-[.toaster]:bg-blue-950 group-[.toaster]:border-blue-900",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
