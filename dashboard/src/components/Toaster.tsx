// Toast notifications.
//
// Radix Themes has no toast component, so this wraps the Radix TOAST PRIMITIVE
// (which plan §11 allows alongside Themes) and renders its content entirely
// from Themes components — Card, Flex, Text, IconButton. The only bespoke
// styling is the fixed viewport and its slide-in, which no Themes component
// can provide; everything visual comes from Radix tokens.

import * as Toast from '@radix-ui/react-toast'
import { Card, Flex, IconButton, Text } from '@radix-ui/themes'
import {
  CheckCircledIcon,
  Cross2Icon,
  CrossCircledIcon,
  InfoCircledIcon,
} from '@radix-ui/react-icons'
import { createContext, useCallback, useContext, useMemo, useState } from 'react'

type Variant = 'success' | 'error' | 'info'

interface ToastItem {
  id: number
  variant: Variant
  title: string
  description?: string
}

interface ToastApi {
  success: (title: string, description?: string) => void
  error: (title: string, description?: string) => void
  info: (title: string, description?: string) => void
}

const ToastContext = createContext<ToastApi | null>(null)

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside <Toaster>')
  return ctx
}

const meta: Record<Variant, { icon: typeof CheckCircledIcon; color: 'green' | 'red' | 'blue' }> = {
  success: { icon: CheckCircledIcon, color: 'green' },
  error: { icon: CrossCircledIcon, color: 'red' },
  info: { icon: InfoCircledIcon, color: 'blue' },
}

export function Toaster({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])

  const push = useCallback((variant: Variant, title: string, description?: string) => {
    // Date.now alone can collide when two toasts fire in the same tick.
    const id = Date.now() + Math.random()
    setItems((prev) => [...prev, { id, variant, title, description }])
  }, [])

  const api = useMemo<ToastApi>(
    () => ({
      success: (t, d) => push('success', t, d),
      error: (t, d) => push('error', t, d),
      info: (t, d) => push('info', t, d),
    }),
    [push],
  )

  const dismiss = (id: number) => setItems((prev) => prev.filter((i) => i.id !== id))

  return (
    <ToastContext.Provider value={api}>
      <Toast.Provider swipeDirection="right" duration={5000}>
        {children}

        {items.map((item) => {
          const { icon: Icon, color } = meta[item.variant]
          return (
            <Toast.Root
              key={item.id}
              className="toast-root"
              onOpenChange={(open) => !open && dismiss(item.id)}
            >
              <Card size="2">
                <Flex gap="3" align="start">
                  <Text color={color} style={{ display: 'flex', marginTop: 2 }}>
                    <Icon width="18" height="18" />
                  </Text>

                  <Flex direction="column" gap="1" style={{ minWidth: 0, flex: 1 }}>
                    <Toast.Title asChild>
                      <Text size="2" weight="medium">
                        {item.title}
                      </Text>
                    </Toast.Title>
                    {item.description && (
                      <Toast.Description asChild>
                        <Text size="1" color="gray">
                          {item.description}
                        </Text>
                      </Toast.Description>
                    )}
                  </Flex>

                  <Toast.Close asChild>
                    <IconButton size="1" variant="ghost" color="gray" aria-label="Dismiss">
                      <Cross2Icon />
                    </IconButton>
                  </Toast.Close>
                </Flex>
              </Card>
            </Toast.Root>
          )
        })}

        <Toast.Viewport className="toast-viewport" />
      </Toast.Provider>
    </ToastContext.Provider>
  )
}
