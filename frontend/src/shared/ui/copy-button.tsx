import { CheckIcon, CopyIcon } from "lucide-react"
import { useEffect, useState } from "react"
import { Button } from "./button"

/**
 * Copies `value` to the clipboard. The button has an accessible name of its own
 * (`label`); the result is announced in a polite live region (WCAG 4.1.3).
 */
export function CopyButton({
  value,
  label,
  copiedText,
  failedText,
}: {
  value: string
  label: string
  copiedText: string
  failedText: string
}) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle")
  useEffect(() => {
    if (state === "idle") return
    const id = setTimeout(() => setState("idle"), 2000)
    return () => clearTimeout(id)
  }, [state])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setState("copied")
    } catch {
      setState("failed")
    }
  }

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="icon-sm"
        aria-label={label}
        onClick={() => void copy()}
      >
        {state === "copied" ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
      </Button>
      <span role="status" className="sr-only">
        {state === "copied" ? copiedText : state === "failed" ? failedText : ""}
      </span>
    </>
  )
}
